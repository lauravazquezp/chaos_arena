package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/chaos-arena/control-plane/api"
	"github.com/chaos-arena/control-plane/arena"
	"github.com/chaos-arena/control-plane/attacks"
	dockerclient "github.com/chaos-arena/control-plane/docker"
	"github.com/chaos-arena/control-plane/metrics"
	"github.com/chaos-arena/control-plane/reconciler"
	"github.com/chaos-arena/control-plane/ws"
)

func main() {
	configPath := os.Getenv("ARENA_CONFIG")
	if configPath == "" {
		configPath = "arena.yaml"
	}
	reportsDir := os.Getenv("REPORTS_DIR")
	if reportsDir == "" {
		reportsDir = "./reports"
	}

	cfg, err := arena.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	state := arena.NewArenaState()
	for _, svc := range cfg.Arena.Services {
		state.Services[svc.ID] = &arena.ServiceState{
			ID:     svc.ID,
			Status: arena.StatusUnknown,
		}
	}

	dockerCli, err := dockerclient.NewDockerClient()
	if err != nil {
		log.Fatalf("docker client: %v", err)
	}

	hub := ws.NewHub()
	m := metrics.NewMetrics()

	dispatcher := attacks.NewDispatcher(dockerCli, state, hub)
	router := api.NewRouter(state, dispatcher, hub, m, reportsDir, cfg.Game.DurationSeconds)

	loop := reconciler.NewLoop(state, dockerCli, hub, m)
	loop.OnHealed = router.OnHealed
	ctx := context.Background()
	go loop.Run(ctx)

	mux := http.NewServeMux()
	router.Register(mux)

	log.Println("control-plane listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}
