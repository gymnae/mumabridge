package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gymnae/mumabridge/internal/bridge"
	"github.com/gymnae/mumabridge/internal/config"
	"github.com/gymnae/mumabridge/internal/health"
	"github.com/gymnae/mumabridge/internal/livekit"
	matrixbridge "github.com/gymnae/mumabridge/internal/matrix"
	"github.com/gymnae/mumabridge/internal/mumble"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		client := http.Client{Timeout: 3 * time.Second}
		response, err := client.Get("http://127.0.0.1:8080/healthz")
		if err != nil || response.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		_ = response.Body.Close()
		return
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(2)
	}

	level := new(slog.LevelVar)
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		slog.Error("invalid log level", "error", err)
		os.Exit(2)
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(log)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	matrixSource := bridge.NewMatrixSource(cfg.Matrix.RoomID, cfg.Matrix.GhostPrefix)
	applicationService := matrixbridge.NewAppService(cfg.Matrix.HSToken, matrixSource.Handle)
	healthServer := health.New(cfg.HTTP.Address, applicationService.Handler())
	matrixClient := matrixbridge.NewClient(cfg.Matrix.HomeserverURL, cfg.Matrix.ServerName, cfg.Matrix.ASToken)
	liveKitManager := livekit.NewManager(livekit.Config{URL: cfg.LiveKit.URL, APIKey: cfg.LiveKit.APIKey, APISecret: cfg.LiveKit.APISecret, Room: cfg.LiveKit.Room}, livekit.NewSDKDialer())
	mumbleManager := mumble.NewManager(mumble.Config{Address: cfg.Mumble.Address, Channel: cfg.Mumble.Channel, Password: cfg.Mumble.Password, GhostPrefix: cfg.Mumble.GhostPrefix, InsecureTLS: cfg.Mumble.InsecureTLS}, mumble.NewGumbleDialer())
	factory := &bridge.GhostFactoryAdapter{Matrix: matrixClient, LiveKit: liveKitManager, Mumble: mumbleManager, RoomID: cfg.Matrix.RoomID, GhostPrefix: cfg.Matrix.GhostPrefix}
	mumbleSource := &bridge.MumbleSource{Dialer: mumble.NewGumbleDialer(), Address: cfg.Mumble.Address, Username: cfg.Mumble.Username, Password: cfg.Mumble.Password, Channel: cfg.Mumble.Channel, GhostPrefix: cfg.Mumble.GhostPrefix, InsecureTLS: cfg.Mumble.InsecureTLS}
	service := bridge.New(log, bridge.NewReconciler(factory, cfg.Limits.Participants), matrixSource, mumbleSource)

	errCh := make(chan error, 2)
	go func() { errCh <- healthServer.ListenAndServe() }()
	go func() { errCh <- service.Run(ctx) }()
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				healthServer.SetReady(service.Ready())
			}
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			log.Error("service failed", "error", err)
		}
		cancel()
	}

	healthServer.SetReady(false)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		log.Error("health server shutdown failed", "error", err)
	}
}
