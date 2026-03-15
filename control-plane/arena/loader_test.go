package arena

import (
	"os"
	"testing"
)

func TestLoadConfig_Valid(t *testing.T) {
	content := `
arena:
  name: "test-arena"
  services:
    - id: service-a
      depends_on: [service-b]
    - id: service-b
      depends_on: []
game:
  duration_seconds: 60
  auto_attacks:
    - at_second: 10
      service: service-a
      attack: kill_switch
`
	f := writeTempFile(t, content)
	cfg, err := LoadConfig(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Arena.Name != "test-arena" {
		t.Errorf("expected arena name 'test-arena', got %q", cfg.Arena.Name)
	}
	if len(cfg.Arena.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(cfg.Arena.Services))
	}
}

func TestLoadConfig_MissingServices(t *testing.T) {
	content := `
arena:
  name: "test-arena"
game:
  duration_seconds: 60
`
	f := writeTempFile(t, content)
	_, err := LoadConfig(f)
	if err == nil {
		t.Fatal("expected error for missing services, got nil")
	}
}

func TestLoadConfig_UnknownAttackType(t *testing.T) {
	content := `
arena:
  name: "test-arena"
  services:
    - id: service-a
      depends_on: []
game:
  duration_seconds: 60
  auto_attacks:
    - at_second: 10
      service: service-a
      attack: nuke_everything
`
	f := writeTempFile(t, content)
	_, err := LoadConfig(f)
	if err == nil {
		t.Fatal("expected error for unknown attack type, got nil")
	}
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "arena*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}
