package attacks

import (
	"context"
	"fmt"
	"time"

	"github.com/chaos-arena/control-plane/arena"
	"github.com/chaos-arena/control-plane/docker"
	"github.com/chaos-arena/control-plane/ws"
)

// attackDetail describes what actually happens for each attack type.
var attackDetail = map[arena.AttackType]string{
	arena.AttackKillSwitch:   "docker stop <container>",
	arena.AttackBlackHole:    "iptables -I INPUT -j DROP  (drops all inbound traffic)",
	arena.AttackResourceHog:  "stress-ng --cpu 4 --timeout 30s  (saturates CPUs for 30s)",
	arena.AttackLatencySpike: "tc qdisc add dev eth0 root netem delay 500ms  (adds 500ms to all network I/O)",
}

type Dispatcher struct {
	docker *docker.DockerClient
	state  *arena.ArenaState
	hub    *ws.Hub
}

func NewDispatcher(d *docker.DockerClient, s *arena.ArenaState, h *ws.Hub) *Dispatcher {
	return &Dispatcher{docker: d, state: s, hub: h}
}

func (d *Dispatcher) Apply(ctx context.Context, serviceName string, attackType arena.AttackType) error {
	containerID, err := d.docker.GetContainerID(ctx, serviceName)
	if err != nil {
		return fmt.Errorf("resolving container for %s: %w", serviceName, err)
	}

	switch attackType {
	case arena.AttackKillSwitch:
		err = d.docker.StopContainer(ctx, containerID)
	case arena.AttackBlackHole:
		_, err = d.docker.ExecInContainer(ctx, containerID, []string{"iptables", "-I", "INPUT", "-j", "DROP"})
	case arena.AttackResourceHog:
		_, err = d.docker.ExecInContainer(ctx, containerID, []string{"stress-ng", "--cpu", "4", "--timeout", "30s"})
	case arena.AttackLatencySpike:
		_, err = d.docker.ExecInContainer(ctx, containerID, []string{"tc", "qdisc", "add", "dev", "eth0", "root", "netem", "delay", "500ms"})
	default:
		return fmt.Errorf("unknown attack type: %s", attackType)
	}

	if err != nil {
		return fmt.Errorf("applying attack %s to %s: %w", attackType, serviceName, err)
	}

	now := time.Now()
	d.state.Lock()
	if svc, ok := d.state.Services[serviceName]; ok {
		svc.ActiveAttack = &attackType
		svc.AttackedAt = &now
		svc.ContainerID = containerID
	}
	d.state.Unlock()

	d.hub.Broadcast(ws.Event{
		Type: ws.EventContainerDying,
		Payload: map[string]string{
			"service": serviceName,
			"attack":  string(attackType),
			"detail":  attackDetail[attackType],
		},
	})

	return nil
}

func (d *Dispatcher) Undo(ctx context.Context, serviceName string, attackType arena.AttackType, containerID string) error {
	var err error
	switch attackType {
	case arena.AttackKillSwitch:
		err = d.docker.StartContainer(ctx, containerID)
	case arena.AttackBlackHole:
		_, err = d.docker.ExecInContainer(ctx, containerID, []string{"iptables", "-F"})
	case arena.AttackResourceHog:
		_, err = d.docker.ExecInContainer(ctx, containerID, []string{"pkill", "stress-ng"})
	case arena.AttackLatencySpike:
		_, err = d.docker.ExecInContainer(ctx, containerID, []string{"tc", "qdisc", "del", "dev", "eth0", "root"})
	default:
		return fmt.Errorf("unknown attack type: %s", attackType)
	}
	return err
}
