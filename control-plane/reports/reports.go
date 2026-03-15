package reports

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chaos-arena/control-plane/arena"
)

type GameEvent struct {
	Service    string         `json:"service"`
	Attack     arena.AttackType `json:"attack"`
	AttackedAt time.Time      `json:"attacked_at"`
	DetectedAt time.Time      `json:"detected_at"`
	HealedAt   *time.Time     `json:"healed_at,omitempty"`
	MTTRMs     *int64         `json:"mttr_ms,omitempty"`
	Outcome    string         `json:"outcome"` // "healed" | "failed" | "in_progress"
}

type Summary struct {
	TotalAttacks     int      `json:"total_attacks"`
	SuccessfulHeals  int      `json:"successful_heals"`
	FailedHeals      int      `json:"failed_heals"`
	MeanMTTRMs       float64  `json:"mean_mttr_ms"`
	ServicesAffected []string `json:"services_affected"`
}

type PostMortem struct {
	GameID    string      `json:"game_id"`
	Mode      string      `json:"mode"`
	StartedAt time.Time   `json:"started_at"`
	EndedAt   time.Time   `json:"ended_at"`
	Events    []GameEvent `json:"events"`
	Summary   Summary     `json:"summary"`
}

func Generate(gameID string, startedAt time.Time, events []GameEvent) *PostMortem {
	now := time.Now()

	summary := summarize(events)

	return &PostMortem{
		GameID:    gameID,
		Mode:      "auto",
		StartedAt: startedAt,
		EndedAt:   now,
		Events:    events,
		Summary:   summary,
	}
}

func summarize(events []GameEvent) Summary {
	affected := map[string]bool{}
	var totalMTTR int64
	var healCount int
	var failedCount int

	for _, e := range events {
		affected[e.Service] = true
		switch e.Outcome {
		case "healed":
			healCount++
			if e.MTTRMs != nil {
				totalMTTR += *e.MTTRMs
			}
		case "failed":
			failedCount++
		}
	}

	var meanMTTR float64
	if healCount > 0 {
		meanMTTR = float64(totalMTTR) / float64(healCount)
	}

	services := make([]string, 0, len(affected))
	for s := range affected {
		services = append(services, s)
	}

	return Summary{
		TotalAttacks:     len(events),
		SuccessfulHeals:  healCount,
		FailedHeals:      failedCount,
		MeanMTTRMs:       meanMTTR,
		ServicesAffected: services,
	}
}

func Write(reportsDir string, pm *PostMortem) error {
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return fmt.Errorf("creating reports dir: %w", err)
	}
	path := filepath.Join(reportsDir, pm.GameID+".json")
	data, err := json.MarshalIndent(pm, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling post-mortem: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func ListReports(reportsDir string) ([]string, error) {
	entries, err := os.ReadDir(reportsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func ReadReport(reportsDir, gameID string) (*PostMortem, error) {
	path := filepath.Join(reportsDir, gameID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading report: %w", err)
	}
	var pm PostMortem
	if err := json.Unmarshal(data, &pm); err != nil {
		return nil, fmt.Errorf("parsing report: %w", err)
	}
	return &pm, nil
}
