package reports

import (
	"testing"
	"time"

	"github.com/chaos-arena/control-plane/arena"
)

func int64Ptr(v int64) *int64 { return &v }

func TestMeanMTTR(t *testing.T) {
	now := time.Now()
	events := []GameEvent{
		{
			Service:    "service-b",
			Attack:     arena.AttackKillSwitch,
			AttackedAt: now,
			DetectedAt: now,
			MTTRMs:     int64Ptr(4000),
			Outcome:    "healed",
		},
		{
			Service:    "service-c",
			Attack:     arena.AttackLatencySpike,
			AttackedAt: now,
			DetectedAt: now,
			MTTRMs:     int64Ptr(2000),
			Outcome:    "healed",
		},
	}

	pm := Generate("test-game", now, events)

	want := 3000.0
	if pm.Summary.MeanMTTRMs != want {
		t.Errorf("want mean MTTR %v ms, got %v ms", want, pm.Summary.MeanMTTRMs)
	}
	if pm.Summary.TotalAttacks != 2 {
		t.Errorf("want 2 total attacks, got %d", pm.Summary.TotalAttacks)
	}
	if pm.Summary.SuccessfulHeals != 2 {
		t.Errorf("want 2 successful heals, got %d", pm.Summary.SuccessfulHeals)
	}
}

func TestWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	pm := Generate("abc123", now, []GameEvent{
		{
			Service:    "service-a",
			Attack:     arena.AttackBlackHole,
			AttackedAt: now,
			DetectedAt: now,
			MTTRMs:     int64Ptr(1500),
			Outcome:    "healed",
		},
	})

	if err := Write(dir, pm); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := ReadReport(dir, "abc123")
	if err != nil {
		t.Fatalf("ReadReport: %v", err)
	}
	if got.GameID != "abc123" {
		t.Errorf("want GameID abc123, got %q", got.GameID)
	}
	if got.Summary.MeanMTTRMs != 1500.0 {
		t.Errorf("want mean MTTR 1500, got %v", got.Summary.MeanMTTRMs)
	}
}
