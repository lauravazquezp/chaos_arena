package reconciler

import (
	"github.com/chaos-arena/control-plane/arena"
)

type ActionType string

const (
	ActionNone         ActionType = "none"
	ActionUndo         ActionType = "undo"
	ActionMarkHealed   ActionType = "mark_healed"
	ActionMarkFailed   ActionType = "mark_failed"
	ActionClearAttack  ActionType = "clear_attack" // attack self-terminated, service already healthy
)

type DiffResult struct {
	Action     ActionType
	AttackType *arena.AttackType
	EmitHealed bool
}

// Diff computes what action to take given desired vs actual state.
// actualHealthy: true if the health check returned 200.
func Diff(svc *arena.ServiceState, actualHealthy bool) DiffResult {
	// Fully healthy, no lingering attack marker — nothing to do.
	if actualHealthy && svc.Status == arena.StatusHealthy && svc.ActiveAttack == nil {
		return DiffResult{Action: ActionNone}
	}

	// Provisioning → healthy: the Undo has taken effect.
	if actualHealthy && svc.Status == arena.StatusProvisioning {
		return DiffResult{Action: ActionMarkHealed, EmitHealed: true}
	}

	// Healthy but attack marker is still set (e.g. resource_hog self-terminated after 30s,
	// or latency_spike was manually cleared externally).
	if actualHealthy && svc.ActiveAttack != nil {
		at := *svc.ActiveAttack
		return DiffResult{Action: ActionClearAttack, AttackType: &at}
	}

	// Unexpected failure — not caused by a known attack.
	if !actualHealthy && svc.Status == arena.StatusHealthy && svc.ActiveAttack == nil {
		return DiffResult{Action: ActionMarkFailed}
	}

	// We caused this failure — auto-heal where possible.
	if !actualHealthy && svc.ActiveAttack != nil {
		at := *svc.ActiveAttack
		switch at {
		case arena.AttackKillSwitch, arena.AttackBlackHole, arena.AttackLatencySpike:
			return DiffResult{Action: ActionUndo, AttackType: &at}
		// resource_hog: stress-ng self-terminates after 30s; service stays
		// responsive, so this branch is never hit for resource_hog.
		// When stress-ng finishes the next tick catches it via ActionClearAttack.
		}
	}

	return DiffResult{Action: ActionNone}
}
