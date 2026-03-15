package arena

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type ServiceConfig struct {
	ID        string   `yaml:"id"`
	DependsOn []string `yaml:"depends_on"`
}

type AutoAttack struct {
	AtSecond int        `yaml:"at_second"`
	Service  string     `yaml:"service"`
	Attack   AttackType `yaml:"attack"`
}

type GameConfig struct {
	DurationSeconds int          `yaml:"duration_seconds"`
	AutoAttacks     []AutoAttack `yaml:"auto_attacks"`
}

type ArenaConfig struct {
	Name     string          `yaml:"name"`
	Services []ServiceConfig `yaml:"services"`
}

type Config struct {
	Arena ArenaConfig `yaml:"arena"`
	Game  GameConfig  `yaml:"game"`
}

var validAttackTypes = map[AttackType]bool{
	AttackKillSwitch:   true,
	AttackBlackHole:    true,
	AttackResourceHog:  true,
	AttackLatencySpike: true,
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config YAML: %w", err)
	}

	if len(cfg.Arena.Services) == 0 {
		return nil, fmt.Errorf("config missing required field: arena.services")
	}

	for _, aa := range cfg.Game.AutoAttacks {
		if !validAttackTypes[aa.Attack] {
			return nil, fmt.Errorf("unknown attack type %q in auto_attacks", aa.Attack)
		}
	}

	return &cfg, nil
}
