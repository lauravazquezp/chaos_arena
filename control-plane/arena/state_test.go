package arena

import (
	"testing"
)

func TestNewArenaState(t *testing.T) {
	s := NewArenaState()
	if s.Services == nil {
		t.Fatal("expected Services map to be initialized")
	}
}

func TestArenaStateConcurrentAccess(t *testing.T) {
	s := NewArenaState()
	s.Lock()
	s.Services["svc"] = &ServiceState{ID: "svc", Status: StatusHealthy}
	s.Unlock()

	s.RLock()
	svc, ok := s.Services["svc"]
	s.RUnlock()

	if !ok {
		t.Fatal("expected service to exist")
	}
	if svc.Status != StatusHealthy {
		t.Errorf("expected healthy, got %q", svc.Status)
	}
}
