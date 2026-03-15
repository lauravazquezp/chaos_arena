package reconciler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/chaos-arena/control-plane/arena"
	"github.com/chaos-arena/control-plane/docker"
	"github.com/chaos-arena/control-plane/metrics"
	"github.com/chaos-arena/control-plane/ws"
)

var undoDetail = map[arena.AttackType]string{
	arena.AttackKillSwitch:   "docker start <container>",
	arena.AttackBlackHole:    "iptables -F  (flushing all rules)",
	arena.AttackResourceHog:  "stress-ng terminated (30s self-timeout)",
	arena.AttackLatencySpike: "tc qdisc del dev eth0 root  (removing netem qdisc)",
}

// HealRecord is passed to the OnHealed callback so callers can record MTTR.
type HealRecord struct {
	ServiceID  string
	AttackType arena.AttackType
	HealedAt   time.Time
	MTTRMs     int64
}

type Loop struct {
	state      *arena.ArenaState
	docker     *docker.DockerClient
	hub        *ws.Hub
	metrics    *metrics.Metrics
	httpClient *http.Client

	// OnHealed is called (from the loop goroutine) each time a service is confirmed healthy.
	OnHealed func(HealRecord)
}

func NewLoop(s *arena.ArenaState, d *docker.DockerClient, h *ws.Hub, m *metrics.Metrics) *Loop {
	return &Loop{
		state:   s,
		docker:  d,
		hub:     h,
		metrics: m,
		httpClient: &http.Client{
			Timeout: 200 * time.Millisecond,
		},
	}
}

func (l *Loop) Run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.tick(ctx)
		}
	}
}

func (l *Loop) tick(ctx context.Context) {
	l.state.RLock()
	serviceIDs := make([]string, 0, len(l.state.Services))
	for id := range l.state.Services {
		serviceIDs = append(serviceIDs, id)
	}
	l.state.RUnlock()

	// Collect per-service health findings for the tick summary.
	type finding struct {
		Service string `json:"service"`
		Healthy bool   `json:"healthy"`
		Status  string `json:"status"`
		Attack  string `json:"attack,omitempty"`
	}
	findings := make([]finding, 0, len(serviceIDs))

	for _, id := range serviceIDs {
		healthy := l.checkHealth(id)

		l.state.RLock()
		svc, ok := l.state.Services[id]
		var f finding
		if ok {
			f = finding{
				Service: id,
				Healthy: healthy,
				Status:  string(svc.Status),
			}
			if svc.ActiveAttack != nil {
				f.Attack = string(*svc.ActiveAttack)
			}
		}
		l.state.RUnlock()

		findings = append(findings, f)
		l.reconcileServiceWithResult(ctx, id, healthy)
	}

	l.hub.Broadcast(ws.Event{
		Type:    ws.EventReconcilerTick,
		Payload: findings,
	})
}

// reconcileServiceWithResult runs reconciliation for a service using a
// pre-computed health check result (avoids a second HTTP call).
func (l *Loop) reconcileServiceWithResult(ctx context.Context, serviceID string, actualHealthy bool) {
	l.state.RLock()
	svc, ok := l.state.Services[serviceID]
	if !ok {
		l.state.RUnlock()
		return
	}
	svcCopy := *svc
	l.state.RUnlock()

	l.applyDiff(ctx, serviceID, &svcCopy, actualHealthy)
}

func (l *Loop) applyDiff(ctx context.Context, serviceID string, svcCopy *arena.ServiceState, actualHealthy bool) {
	result := Diff(svcCopy, actualHealthy)

	switch result.Action {
	case ActionNone:
		return

	case ActionMarkFailed:
		now := time.Now()
		l.state.Lock()
		if s, ok := l.state.Services[serviceID]; ok {
			s.Status = arena.StatusUnhealthy
			s.DetectedAt = &now
		}
		l.state.Unlock()
		l.hub.Broadcast(ws.Event{
			Type: ws.EventContainerDying,
			Payload: map[string]string{
				"service": serviceID,
				"detail":  "unexpected failure (no active attack)",
			},
		})

	case ActionUndo:
		if result.AttackType == nil {
			return
		}
		containerID := svcCopy.ContainerID
		if containerID == "" {
			var err error
			containerID, err = l.docker.GetContainerID(ctx, serviceID)
			if err != nil {
				log.Printf("reconciler: get container id for %s: %v", serviceID, err)
				return
			}
		}
		if err := l.undoAttack(ctx, serviceID, *result.AttackType, containerID); err != nil {
			log.Printf("reconciler: undo attack on %s: %v", serviceID, err)
			l.hub.Broadcast(ws.Event{
				Type: ws.EventControlPlaneError,
				Payload: map[string]string{"service": serviceID, "error": err.Error()},
			})
			return
		}
		now := time.Now()
		l.state.Lock()
		if s, ok := l.state.Services[serviceID]; ok {
			s.Status = arena.StatusProvisioning
			if s.DetectedAt == nil {
				s.DetectedAt = &now
			}
		}
		l.state.Unlock()
		l.hub.Broadcast(ws.Event{
			Type: ws.EventContainerProvisioning,
			Payload: map[string]string{
				"service": serviceID,
				"attack":  string(*result.AttackType),
				"detail":  undoDetail[*result.AttackType],
			},
		})

	case ActionMarkHealed:
		now := time.Now()
		var mttrMs int64
		var attackType arena.AttackType
		l.state.Lock()
		if s, ok := l.state.Services[serviceID]; ok {
			if s.DetectedAt != nil {
				mttrMs = now.Sub(*s.DetectedAt).Milliseconds()
			}
			if s.ActiveAttack != nil {
				attackType = *s.ActiveAttack
			}
			s.Status = arena.StatusHealthy
			s.ActiveAttack = nil
			s.AttackedAt = nil
			s.DetectedAt = nil
		}
		l.state.Unlock()

		if l.metrics != nil && result.EmitHealed {
			l.metrics.IncrementHeals(serviceID, "healed")
			l.metrics.SetMTTR(serviceID, mttrMs)
		}
		if result.EmitHealed {
			l.hub.Broadcast(ws.Event{
				Type: ws.EventContainerHealthy,
				Payload: map[string]interface{}{
					"service": serviceID,
					"attack":  string(attackType),
					"mttr_ms": mttrMs,
					"detail":  fmt.Sprintf("health check passed after %dms", mttrMs),
				},
			})
		}
		if l.OnHealed != nil {
			l.OnHealed(HealRecord{ServiceID: serviceID, AttackType: attackType, HealedAt: now, MTTRMs: mttrMs})
		}

	case ActionClearAttack:
		// Attack self-terminated (e.g. resource_hog after 30s).
		if result.AttackType != nil {
			containerID := svcCopy.ContainerID
			if containerID == "" {
				if id, err := l.docker.GetContainerID(ctx, serviceID); err == nil {
					containerID = id
				}
			}
			if containerID != "" {
				_ = l.undoAttack(ctx, serviceID, *result.AttackType, containerID)
			}
		}
		now := time.Now()
		var mttrMs int64
		var attackType arena.AttackType
		l.state.Lock()
		if s, ok := l.state.Services[serviceID]; ok {
			if s.AttackedAt != nil {
				mttrMs = now.Sub(*s.AttackedAt).Milliseconds()
			}
			if s.ActiveAttack != nil {
				attackType = *s.ActiveAttack
			}
			s.ActiveAttack = nil
			s.AttackedAt = nil
			s.DetectedAt = nil
		}
		l.state.Unlock()
		if l.metrics != nil && attackType != "" {
			l.metrics.IncrementHeals(serviceID, "healed")
			l.metrics.SetMTTR(serviceID, mttrMs)
		}
		detail := undoDetail[attackType]
		if detail == "" {
			detail = "attack self-terminated"
		}
		l.hub.Broadcast(ws.Event{
			Type: ws.EventContainerHealthy,
			Payload: map[string]interface{}{
				"service": serviceID,
				"attack":  string(attackType),
				"mttr_ms": mttrMs,
				"detail":  detail,
			},
		})
		if l.OnHealed != nil && attackType != "" {
			l.OnHealed(HealRecord{ServiceID: serviceID, AttackType: attackType, HealedAt: now, MTTRMs: mttrMs})
		}
	}
}

func (l *Loop) checkHealth(serviceID string) bool {
	url := fmt.Sprintf("http://%s:3000/health", serviceID)
	resp, err := l.httpClient.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (l *Loop) undoAttack(ctx context.Context, serviceID string, attackType arena.AttackType, containerID string) error {
	switch attackType {
	case arena.AttackKillSwitch:
		return l.docker.StartContainer(ctx, containerID)
	case arena.AttackBlackHole:
		_, err := l.docker.ExecInContainer(ctx, containerID, []string{"iptables", "-F"})
		return err
	case arena.AttackResourceHog:
		_, err := l.docker.ExecInContainer(ctx, containerID, []string{"pkill", "stress-ng"})
		return err
	case arena.AttackLatencySpike:
		_, err := l.docker.ExecInContainer(ctx, containerID, []string{"tc", "qdisc", "del", "dev", "eth0", "root"})
		return err
	default:
		return fmt.Errorf("unknown attack type: %s", attackType)
	}
}
