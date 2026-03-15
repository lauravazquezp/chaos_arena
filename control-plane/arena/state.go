package arena

import (
	"sync"
	"time"
)

type ServiceStatus string

const (
	StatusHealthy      ServiceStatus = "healthy"
	StatusUnhealthy    ServiceStatus = "unhealthy"
	StatusProvisioning ServiceStatus = "provisioning"
	StatusUnknown      ServiceStatus = "unknown"
)

type AttackType string

const (
	AttackKillSwitch   AttackType = "kill_switch"
	AttackBlackHole    AttackType = "black_hole"
	AttackResourceHog  AttackType = "resource_hog"
	AttackLatencySpike AttackType = "latency_spike"
)

type ServiceState struct {
	ID           string        `json:"id"`
	Status       ServiceStatus `json:"status"`
	ActiveAttack *AttackType   `json:"active_attack,omitempty"`
	ContainerID  string        `json:"container_id"`
	AttackedAt   *time.Time    `json:"attacked_at,omitempty"`
	DetectedAt   *time.Time    `json:"detected_at,omitempty"`
}

type ArenaState struct {
	mu       sync.RWMutex
	Services map[string]*ServiceState
}

func NewArenaState() *ArenaState {
	return &ArenaState{
		Services: make(map[string]*ServiceState),
	}
}

func (a *ArenaState) RLock()   { a.mu.RLock() }
func (a *ArenaState) RUnlock() { a.mu.RUnlock() }
func (a *ArenaState) Lock()    { a.mu.Lock() }
func (a *ArenaState) Unlock()  { a.mu.Unlock() }
