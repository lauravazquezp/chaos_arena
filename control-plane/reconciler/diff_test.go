package reconciler

import (
	"testing"
	"time"

	"github.com/chaos-arena/control-plane/arena"
)

func ptr[T any](v T) *T { return &v }

func TestDiff(t *testing.T) {
	now := time.Now()
	ks := arena.AttackKillSwitch
	bh := arena.AttackBlackHole
	rh := arena.AttackResourceHog
	ls := arena.AttackLatencySpike

	tests := []struct {
		name          string
		svc           *arena.ServiceState
		actualHealthy bool
		wantAction    ActionType
		wantEmit      bool
	}{
		{
			name:          "healthy with no attack — no action",
			svc:           &arena.ServiceState{Status: arena.StatusHealthy},
			actualHealthy: true,
			wantAction:    ActionNone,
		},
		{
			name: "unhealthy with kill_switch active — undo",
			svc: &arena.ServiceState{
				Status:       arena.StatusUnhealthy,
				ActiveAttack: &ks,
				DetectedAt:   &now,
			},
			actualHealthy: false,
			wantAction:    ActionUndo,
		},
		{
			name: "unhealthy with black_hole active — undo",
			svc: &arena.ServiceState{
				Status:       arena.StatusUnhealthy,
				ActiveAttack: &bh,
				DetectedAt:   &now,
			},
			actualHealthy: false,
			wantAction:    ActionUndo,
		},
		{
			name: "unhealthy with latency_spike active — undo",
			svc: &arena.ServiceState{
				Status:       arena.StatusUnhealthy,
				ActiveAttack: &ls,
				DetectedAt:   &now,
			},
			actualHealthy: false,
			wantAction:    ActionUndo,
		},
		{
			name:          "provisioning becomes healthy — mark healed",
			svc:           &arena.ServiceState{Status: arena.StatusProvisioning, DetectedAt: &now},
			actualHealthy: true,
			wantAction:    ActionMarkHealed,
			wantEmit:      true,
		},
		{
			name:          "unexpected failure — mark failed",
			svc:           &arena.ServiceState{Status: arena.StatusHealthy},
			actualHealthy: false,
			wantAction:    ActionMarkFailed,
		},
		{
			name: "resource_hog self-terminated — clear attack",
			svc: &arena.ServiceState{
				Status:       arena.StatusHealthy,
				ActiveAttack: &rh,
				AttackedAt:   &now,
			},
			actualHealthy: true,
			wantAction:    ActionClearAttack,
		},
		{
			name: "healthy with stale attack marker — clear attack",
			svc: &arena.ServiceState{
				Status:       arena.StatusHealthy,
				ActiveAttack: &ks,
				AttackedAt:   &now,
			},
			actualHealthy: true,
			wantAction:    ActionClearAttack,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := Diff(tc.svc, tc.actualHealthy)
			if result.Action != tc.wantAction {
				t.Errorf("want action %q, got %q", tc.wantAction, result.Action)
			}
			if result.EmitHealed != tc.wantEmit {
				t.Errorf("want EmitHealed=%v, got %v", tc.wantEmit, result.EmitHealed)
			}
		})
	}
}
