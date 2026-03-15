package metrics

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/chaos-arena/control-plane/arena"
)

type attackKey struct {
	service    string
	attackType string
}

type healKey struct {
	service string
	outcome string
}

type Metrics struct {
	mu           sync.RWMutex
	attacksTotal map[attackKey]*atomic.Int64
	healsTotal   map[healKey]*atomic.Int64
	mttrGauge    map[string]*atomic.Int64
}

func NewMetrics() *Metrics {
	return &Metrics{
		attacksTotal: make(map[attackKey]*atomic.Int64),
		healsTotal:   make(map[healKey]*atomic.Int64),
		mttrGauge:    make(map[string]*atomic.Int64),
	}
}

func (m *Metrics) IncrementAttacks(service string, attackType arena.AttackType) {
	k := attackKey{service: service, attackType: string(attackType)}
	m.mu.Lock()
	if _, ok := m.attacksTotal[k]; !ok {
		m.attacksTotal[k] = &atomic.Int64{}
	}
	counter := m.attacksTotal[k]
	m.mu.Unlock()
	counter.Add(1)
}

func (m *Metrics) IncrementHeals(service, outcome string) {
	k := healKey{service: service, outcome: outcome}
	m.mu.Lock()
	if _, ok := m.healsTotal[k]; !ok {
		m.healsTotal[k] = &atomic.Int64{}
	}
	counter := m.healsTotal[k]
	m.mu.Unlock()
	counter.Add(1)
}

func (m *Metrics) SetMTTR(service string, ms int64) {
	m.mu.Lock()
	if _, ok := m.mttrGauge[service]; !ok {
		m.mttrGauge[service] = &atomic.Int64{}
	}
	gauge := m.mttrGauge[service]
	m.mu.Unlock()
	gauge.Store(ms)
}

func (m *Metrics) TextFormat(healthyCount int) string {
	var sb strings.Builder

	sb.WriteString("# HELP chaos_arena_attacks_total Total number of attacks triggered\n")
	sb.WriteString("# TYPE chaos_arena_attacks_total counter\n")
	m.mu.RLock()
	for k, v := range m.attacksTotal {
		fmt.Fprintf(&sb, "chaos_arena_attacks_total{service=%q,attack_type=%q} %d\n",
			k.service, k.attackType, v.Load())
	}

	sb.WriteString("# HELP chaos_arena_heals_total Total number of heal attempts\n")
	sb.WriteString("# TYPE chaos_arena_heals_total counter\n")
	for k, v := range m.healsTotal {
		fmt.Fprintf(&sb, "chaos_arena_heals_total{service=%q,outcome=%q} %d\n",
			k.service, k.outcome, v.Load())
	}

	sb.WriteString("# HELP chaos_arena_mttr_milliseconds Last observed MTTR per service\n")
	sb.WriteString("# TYPE chaos_arena_mttr_milliseconds gauge\n")
	for svc, v := range m.mttrGauge {
		fmt.Fprintf(&sb, "chaos_arena_mttr_milliseconds{service=%q} %d\n", svc, v.Load())
	}
	m.mu.RUnlock()

	sb.WriteString("# HELP chaos_arena_services_healthy Number of currently healthy services\n")
	sb.WriteString("# TYPE chaos_arena_services_healthy gauge\n")
	fmt.Fprintf(&sb, "chaos_arena_services_healthy %d\n", healthyCount)

	return sb.String()
}
