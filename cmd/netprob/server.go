//go:build !agentonly

package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"netprob/internal/auth"
	"netprob/internal/buildinfo"
	"netprob/internal/config"
	"netprob/internal/scheduler"
	"netprob/internal/server"
	"netprob/internal/storage"
	"netprob/internal/webui"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func runServer(cfg *config.Config) {
	// Setup logging
	level := zerolog.InfoLevel
	if cfg.LogLevel == "debug" {
		level = zerolog.DebugLevel
	}
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger().Level(level)

	// Ensure data directory exists
	if err := os.MkdirAll(cfg.Server.DataDir, 0755); err != nil {
		log.Fatal().Err(err).Msg("failed to create data directory")
	}

	// Initialize SQLite
	dsn := filepath.Join(cfg.Server.DataDir, "netprob.db")
	store := storage.NewStore(dsn, nil)
	if err := store.DB.Open(); err != nil {
		log.Fatal().Err(err).Msg("failed to open SQLite database")
	}
	defer store.DB.DB().Close()

	if err := store.DB.ApplyMigrations(store.DB.DB()); err != nil {
		log.Fatal().Err(err).Msg("failed to apply migrations")
	}
	defaultPasswordHash, err := auth.HashPassword(cfg.Server.AdminPassword)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to hash default admin password")
	}
	if err := store.DB.EnsureDefaultAdmin(cfg.Server.AdminEmail, defaultPasswordHash); err != nil {
		log.Fatal().Err(err).Msg("failed to initialize admin account")
	}

	// Initialize metrics store
	var metricsStore storage.MetricsStore
	switch cfg.Metrics.Backend {
	case "victoriametrics":
		if strings.TrimSpace(cfg.Metrics.URL) == "" {
			log.Fatal().Msg("NETPROB_VM_URL is required when NETPROB_METRICS_BACKEND=victoriametrics")
		}
		metricsStore = storage.NewVMMetricsStore(cfg.Metrics.URL)
		log.Info().Str("url", cfg.Metrics.URL).Msg("using VictoriaMetrics for probe and controller metrics")
	default:
		metricsStore = storage.NewSQLiteMetricsStore(store.DB)
		log.Info().Msg("using SQLite for ping metrics")
	}
	store.Metrics = metricsStore

	// Create agent hub
	hub := server.NewHub(store)

	// Create API server
	apiServer := server.NewAPIServer(store, hub, cfg)

	// Create scheduler
	sched := scheduler.NewScheduler(hub, store.DB)

	// Start scheduler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sched.Start(ctx)
	defer sched.Stop()
	go runRetentionCleanup(ctx, store.DB)
	go runMetricsExport(ctx, store, time.Now().UTC(), buildinfo.Version)

	// Start HTTP server - serve API and embedded UI
	addr := cfg.Server.Listen
	log.Info().Str("addr", addr).Msg("starting NetProb server")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API and WS routes handled by chi router
		if r.URL.Path == "/health" || strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/ws" {
			apiServer.Handler().ServeHTTP(w, r)
			return
		}
		// Everything else serves the embedded UI
		webui.Handler().ServeHTTP(w, r)
	})

	httpServer := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	<-sigCh
	log.Info().Msg("shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	httpServer.Shutdown(shutdownCtx)
}

func runRetentionCleanup(ctx context.Context, store *storage.SQLiteStore) {
	cleanup := func() {
		settings, err := store.GetRetentionSettings()
		if err != nil {
			log.Warn().Err(err).Msg("failed to load retention settings")
			return
		}
		result, err := store.CleanupExpiredProbeHistory(settings, time.Now().UTC())
		if err != nil {
			log.Warn().Err(err).Msg("failed to clean expired probe history")
			return
		}
		if result.PingResultsDeleted > 0 || result.MTRRunsDeleted > 0 {
			log.Info().
				Int64("ping_results_deleted", result.PingResultsDeleted).
				Int64("mtr_runs_deleted", result.MTRRunsDeleted).
				Msg("expired probe history cleaned")
		}
	}

	cleanup()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanup()
		}
	}
}

func runMetricsExport(ctx context.Context, store *storage.Store, startedAt time.Time, controllerVersion string) {
	export := func() {
		if err := store.ExportInventory(startedAt, controllerVersion); err != nil {
			log.Warn().Err(err).Msg("failed to export controller inventory metrics")
		}
	}

	export()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			export()
		}
	}
}
