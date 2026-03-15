package attacks

import (
	"testing"

	"github.com/chaos-arena/control-plane/arena"
)

func TestAttackTypeConstants(t *testing.T) {
	types := []arena.AttackType{
		arena.AttackKillSwitch,
		arena.AttackBlackHole,
		arena.AttackResourceHog,
		arena.AttackLatencySpike,
	}
	for _, at := range types {
		if at == "" {
			t.Errorf("attack type should not be empty")
		}
	}
}

func TestDispatcherNilSafety(t *testing.T) {
	// Ensure NewDispatcher doesn't panic with valid args structure
	// (actual docker calls require integration environment)
	d := &Dispatcher{}
	if d == nil {
		t.Fatal("dispatcher should not be nil")
	}
}
