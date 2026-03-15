package api

import (
	"encoding/json"
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

type Router struct {
	state      *arena.ArenaState
	dispatcher *attacks.Dispatcher
	hub        *ws.Hub
	metrics    *metrics.Metrics
	reportsDir string

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
) *Router {
	return &Router{
		state:      state,
		dispatcher: dispatcher,
		hub:        hub,
		metrics:    m,
		reportsDir: reportsDir,
		gameEvents: []reports.GameEvent{},
		inFlight:   make(map[string]reports.GameEvent),
	}
}

func (r *Router) Register(mux *http.ServeMux) {
	mux.HandleFunc("/game/start", r.cors(r.handleGameStart))
	mux.HandleFunc("/game/stop", r.cors(r.handleGameStop))
	mux.HandleFunc("/game/state", r.cors(r.handleGameState))
	mux.HandleFunc("/attack", r.cors(r.handleAttack))
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

func (r *Router) handleGameStart(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.gameID = uuid.New().String()
	r.gameStart = time.Now()
	r.gameEvents = []reports.GameEvent{}
	r.inFlight = make(map[string]reports.GameEvent)

	r.hub.Broadcast(ws.Event{
		Type:    ws.EventGameStarted,
		Payload: map[string]string{"game_id": r.gameID},
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
