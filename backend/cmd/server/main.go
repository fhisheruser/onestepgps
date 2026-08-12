// Command server is the FleetView backend: it polls the GPS provider in the
// background, merges the cached fleet with per-user preferences and serves the
// result over REST and WebSocket.
//
// Composition happens here and nowhere else — every other package receives its
// dependencies through interfaces, which is what keeps them unit-testable.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"fleetview/internal/config"
	"fleetview/internal/domain"
	"fleetview/internal/platform/logger"
	"fleetview/internal/provider/demo"
	"fleetview/internal/provider/onestepgps"
	"fleetview/internal/repository"
	"fleetview/internal/service"
	"fleetview/internal/transport/httpapi"
	"fleetview/internal/transport/ws"
)

func main() {
	if err := run(); err != nil {
		// The logger may not exist yet, so fail loudly on stderr too.
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	log := logger.New(cfg.Log.Level, cfg.Log.Format)
	log.Info("starting FleetView backend",
		"version", cfg.Version,
		"env", cfg.Env,
		"addr", cfg.HTTPAddr,
		"poll_interval", cfg.Poller.Interval.String())

	if cfg.OneStepGPS.DemoMode {
		log.Warn("running in DEMO MODE with simulated devices — set ONESTEPGPS_API_KEY for live data")
	}
	if cfg.Maps.APIKey == "" {
		log.Warn("GOOGLE_MAPS_API_KEY is not set — the dashboard will render without the map")
	}

	clock := domain.SystemClock{}

	// ---- Infrastructure -----------------------------------------------
	db, err := repository.Open(repository.Options{
		Path:        cfg.Database.Path,
		AutoMigrate: cfg.Database.AutoMigrate,
		LogQueries:  cfg.Database.LogQueries,
		Logger:      log,
	})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if cerr := repository.Close(db); cerr != nil {
			log.Error("closing database failed", "error", cerr)
		}
	}()

	preferenceRepo := repository.NewPreferenceRepository(db)
	iconRepo := repository.NewIconRepository(db)
	historyRepo := repository.NewHistoryRepository(db)

	var provider domain.DeviceProvider
	if cfg.OneStepGPS.DemoMode {
		provider = demo.New(cfg.OneStepGPS.SpeedUnit, clock)
	} else {
		provider = onestepgps.New(onestepgps.Options{
			BaseURL:      cfg.OneStepGPS.BaseURL,
			APIKey:       cfg.OneStepGPS.APIKey,
			Timeout:      cfg.OneStepGPS.Timeout,
			MaxAttempts:  cfg.OneStepGPS.MaxAttempts,
			RetryBackoff: cfg.OneStepGPS.RetryBackoff,
			SpeedUnit:    cfg.OneStepGPS.SpeedUnit,
		}, log)
	}

	// ---- Application ----------------------------------------------------
	cache := service.NewSnapshotCache()

	deviceService := service.NewDeviceService(service.DeviceServiceDeps{
		Cache:            cache,
		Preferences:      preferenceRepo,
		History:          historyRepo,
		Clock:            clock,
		MaxHistoryPoints: cfg.History.MaxPointsPerQuery,
	})

	preferenceService := service.NewPreferenceService(service.PreferenceServiceDeps{
		Repo:             preferenceRepo,
		Icons:            iconRepo,
		Clock:            clock,
		MaxIconBytes:     cfg.Icons.MaxBytes,
		MaxIconsPerUser:  cfg.Icons.MaxPerUser,
		AllowedIconTypes: cfg.Icons.AllowedTypes,
		IconURLPrefix:    "/api/v1/icons/",
	})

	hub := ws.NewHub(ws.Options{
		Builder:        deviceService,
		Logger:         log,
		Clock:          clock,
		AllowedOrigins: cfg.AllowedOrigins,
	})

	poller := service.NewPoller(service.PollerDeps{
		Provider:           provider,
		Cache:              cache,
		History:            historyRepo,
		Publisher:          hub,
		Logger:             log,
		Clock:              clock,
		Interval:           cfg.Poller.Interval,
		Timeout:            cfg.Poller.Timeout,
		FailureThreshold:   cfg.Poller.FailureThreshold,
		HistoryEnabled:     cfg.History.Enabled,
		HistoryMinDistance: cfg.History.MinDistanceMeters,
		HistoryRetention:   cfg.History.Retention,
		HistoryPrune:       cfg.History.PruneInterval,
	})

	// ---- Delivery --------------------------------------------------------
	router := httpapi.NewRouter(httpapi.RouterDeps{
		Handlers: httpapi.NewHandlers(httpapi.HandlerDeps{
			Devices:     deviceService,
			Preferences: preferenceService,
			Poller:      poller,
			Hub:         hub,
			Config:      cfg,
			Logger:      log,
			Clock:       clock,
		}),
		Config: cfg,
		Logger: log,
	})

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: router,
		// WriteTimeout is deliberately absent: it would cut off long-lived
		// WebSocket connections. Per-write deadlines are set in the hub, and
		// ReadHeaderTimeout still protects against slowloris.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// ---- Lifecycle -------------------------------------------------------
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		poller.Run(ctx)
	}()

	serverErr := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("http server: %w", err)
		}
	case <-ctx.Done():
		log.Info("shutdown signal received, draining connections",
			"timeout", cfg.ShutdownTimeout.String())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed, closing forcefully", "error", err)
		_ = server.Close()
	}

	stop()
	waitFor(&wg, 5*time.Second, log)
	log.Info("shutdown complete")
	return nil
}

// waitFor blocks until the WaitGroup drains or the timeout elapses, so a stuck
// background task cannot hang the process forever.
func waitFor(wg *sync.WaitGroup, timeout time.Duration, log *slog.Logger) {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		log.Warn("background workers did not stop in time")
	}
}
