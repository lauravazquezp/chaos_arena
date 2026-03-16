package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/chaos-arena/control-plane/arena"
	"github.com/chaos-arena/control-plane/attacks"
	"github.com/chaos-arena/control-plane/metrics"
	"github.com/chaos-arena/control-plane/reconciler"
	"github.com/chaos-arena/control-plane/reports"
	"github.com/chaos-arena/control-plane/ws"
	"github.com/google/uuid"
)

// ServicePool is the fixed ordered list of available services.
var ServicePool = []string{"api-gateway", "auth-service", "payments", "cache", "worker"}

// SimConfig holds the user-configurable simulation topology.
type SimConfig struct {
	Services               []string   `json:"services"`
	Dependencies           [][]string `json:"dependencies"`
	HealFailureProbability float64    `json:"heal_failure_probability"`
}

func validateSimConfig(cfg SimConfig) error {
	if len(cfg.Services) == 0 || len(cfg.Services) > 5 {
		return fmt.Errorf("services must have 1-5 items")
	}
	valid := make(map[string]bool)
	for _, s := range ServicePool {
		valid[s] = true
	}
	seen := make(map[string]bool)
	for _, s := range cfg.Services {
		if !valid[s] {
			return fmt.Errorf("unknown service %q", s)
		}
		if seen[s] {
			return fmt.Errorf("duplicate service %q", s)
		}
		seen[s] = true
	}
	for _, dep := range cfg.Dependencies {
		if len(dep) != 2 {
			return fmt.Errorf("each dependency must be [from, to]")
		}
		if dep[0] == dep[1] {
			return fmt.Errorf("self-dependency not allowed")
		}
		if !seen[dep[0]] {
			return fmt.Errorf("dependency references unknown service %q", dep[0])
		}
		if !seen[dep[1]] {
			return fmt.Errorf("dependency references unknown service %q", dep[1])
		}
	}
	if cfg.HealFailureProbability < 0 || cfg.HealFailureProbability > 0.8 {
		return fmt.Errorf("heal_failure_probability must be 0.0–0.8")
	}
	return nil
}

type Router struct {
	state        *arena.ArenaState
	dispatcher   *attacks.Dispatcher
	hub          *ws.Hub
	metrics      *metrics.Metrics
	reportsDir   string
	gameDuration int // seconds, from arena.yaml
	simConfig    SimConfig

	// SetHealFailureProbability is wired to the reconciler loop in main.go.
	SetHealFailureProbability func(float64)

	mu sync.Mutex
	// current game
	gameID    string
	gameStart time.Time
	// completed events ready for the post-mortem
	gameEvents []reports.GameEvent
	// in-flight attacks indexed by service name; completed on heal
	inFlight map[string]reports.GameEvent
}

func NewRouter(
	state *arena.ArenaState,
	dispatcher *attacks.Dispatcher,
	hub *ws.Hub,
	m *metrics.Metrics,
	reportsDir string,
	gameDuration int,
	defaultConfig SimConfig,
) *Router {
	return &Router{
		state:        state,
		dispatcher:   dispatcher,
		hub:          hub,
		metrics:      m,
		reportsDir:   reportsDir,
		gameDuration: gameDuration,
		simConfig:    defaultConfig,
		gameEvents:   []reports.GameEvent{},
		inFlight:     make(map[string]reports.GameEvent),
	}
}

func (r *Router) Register(mux *http.ServeMux) {
	mux.HandleFunc("/game/start", r.cors(r.handleGameStart))
	mux.HandleFunc("/game/stop", r.cors(r.handleGameStop))
	mux.HandleFunc("/game/state", r.cors(r.handleGameState))
	mux.HandleFunc("/attack", r.cors(r.handleAttack))
	mux.HandleFunc("/simulation/config", r.cors(r.handleSimConfig))
	mux.HandleFunc("/reports", r.cors(r.handleListReports))
	mux.HandleFunc("/reports/", r.cors(r.handleGetReport))
	mux.HandleFunc("/metrics", r.handleMetrics)
	mux.HandleFunc("/healthz", r.handleHealthz)
	mux.HandleFunc("/ws", r.hub.ServeWS)
}

// OnHealed is called by the reconciler loop whenever a service heals.
// It completes any in-flight GameEvent for that service.
func (r *Router) OnHealed(h reconciler.HealRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.gameID == "" {
		return
	}
	ev, ok := r.inFlight[h.ServiceID]
	if !ok {
		return
	}
	delete(r.inFlight, h.ServiceID)
	healedAt := h.HealedAt
	mttr := h.MTTRMs
	ev.HealedAt = &healedAt
	ev.MTTRMs = &mttr
	ev.Outcome = "healed"
	r.gameEvents = append(r.gameEvents, ev)
}

func (r *Router) cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, req)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (r *Router) handleSimConfig(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.mu.Lock()
		cfg := r.simConfig
		r.mu.Unlock()
		writeJSON(w, http.StatusOK, cfg)
	case http.MethodPost:
		var body SimConfig
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}
		if err := validateSimConfig(body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		r.mu.Lock()
		if r.gameID != "" {
			r.mu.Unlock()
			writeJSON(w, http.StatusConflict, map[string]string{"error": "cannot change config during active simulation"})
			return
		}
		r.simConfig = body
		r.mu.Unlock()

		// Update the live service set immediately so the graph reflects the new topology.
		r.state.Lock()
		r.state.Services = make(map[string]*arena.ServiceState)
		for _, id := range body.Services {
			r.state.Services[id] = &arena.ServiceState{ID: id, Status: arena.StatusUnknown}
		}
		r.state.Unlock()

		if r.SetHealFailureProbability != nil {
			r.SetHealFailureProbability(body.HealFailureProbability)
		}
		r.hub.Broadcast(ws.Event{Type: ws.EventSimConfig, Payload: body})
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) handleGameStart(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	// Reset service statuses for the new game (preserves current config).
	r.state.Lock()
	newServices := make(map[string]*arena.ServiceState)
	for _, id := range r.simConfig.Services {
		newServices[id] = &arena.ServiceState{ID: id, Status: arena.StatusUnknown}
	}
	r.state.Services = newServices
	r.state.Unlock()

	r.gameID = uuid.New().String()
	r.gameStart = time.Now()
	r.gameEvents = []reports.GameEvent{}
	r.inFlight = make(map[string]reports.GameEvent)

	r.hub.Broadcast(ws.Event{
		Type: ws.EventGameStarted,
		Payload: map[string]interface{}{
			"game_id":          r.gameID,
			"started_at":       r.gameStart.UTC().Format(time.RFC3339Nano),
			"duration_seconds": r.gameDuration,
		},
	})

	writeJSON(w, http.StatusOK, map[string]string{"game_id": r.gameID})
}

func (r *Router) handleGameStop(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.gameID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no active game"})
		return
	}

	// Flush any still-in-flight attacks as "failed".
	for _, ev := range r.inFlight {
		ev.Outcome = "failed"
		r.gameEvents = append(r.gameEvents, ev)
	}
	r.inFlight = make(map[string]reports.GameEvent)

	pm := reports.Generate(r.gameID, r.gameStart, r.gameEvents)
	if err := reports.Write(r.reportsDir, pm); err != nil {
		log.Printf("write report: %v", err)
	}

	r.hub.Broadcast(ws.Event{
		Type:    ws.EventGameEnded,
		Payload: map[string]string{"game_id": r.gameID},
	})

	r.gameID = ""
	writeJSON(w, http.StatusOK, pm)
}

func (r *Router) handleGameState(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.state.RLock()
	defer r.state.RUnlock()
	writeJSON(w, http.StatusOK, map[string]interface{}{"services": r.state.Services})
}

func (r *Router) handleAttack(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Service string           `json:"service"`
		Attack  arena.AttackType `json:"attack"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}

	now := time.Now()
	if err := r.dispatcher.Apply(req.Context(), body.Service, body.Attack); err != nil {
		log.Printf("attack error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	r.metrics.IncrementAttacks(body.Service, body.Attack)

	// Record the attack as in-flight for this game session.
	r.mu.Lock()
	if r.gameID != "" {
		r.inFlight[body.Service] = reports.GameEvent{
			Service:    body.Service,
			Attack:     body.Attack,
			AttackedAt: now,
			DetectedAt: now,
			Outcome:    "in_progress",
		}
	}
	r.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (r *Router) handleListReports(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	names, err := reports.ListReports(r.reportsDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, names)
}

func (r *Router) handleGetReport(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	gameID := strings.TrimPrefix(req.URL.Path, "/reports/")
	pm, err := reports.ReadReport(r.reportsDir, gameID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, pm)
}

func (r *Router) handleMetrics(w http.ResponseWriter, req *http.Request) {
	r.state.RLock()
	healthy := 0
	for _, svc := range r.state.Services {
		if svc.Status == arena.StatusHealthy {
			healthy++
		}
	}
	r.state.RUnlock()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.Write([]byte(r.metrics.TextFormat(healthy)))
}

func (r *Router) handleHealthz(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
