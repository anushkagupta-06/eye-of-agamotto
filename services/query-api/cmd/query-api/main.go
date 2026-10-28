package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anushkagupta-06/eye-of-agamotto/services/query-api/internal/clickhouse"
	"github.com/anushkagupta-06/eye-of-agamotto/services/query-api/internal/config"
	"github.com/anushkagupta-06/eye-of-agamotto/services/query-api/internal/handler"
	"github.com/anushkagupta-06/eye-of-agamotto/services/query-api/internal/natsio"
	"github.com/anushkagupta-06/eye-of-agamotto/services/query-api/internal/service"
)

func main() {
	if err := run(); err != nil {
		slog.Error("query-api fatal error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	startCtx, cancelStart := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStart()

	// 1. Connect to ClickHouse as query_api reader
	chClient, err := clickhouse.Open(startCtx, cfg)
	if err != nil {
		slog.Warn("ClickHouse reader not reachable at startup; queries will retry", "addr", cfg.ClickHouseAddr, "err", err)
	} else {
		defer chClient.Close()
		slog.Info("connected to ClickHouse storage", "addr", cfg.ClickHouseAddr, "user", cfg.ClickHouseUser)
	}

	// 2. Initialize NATS Live Log Tailer (optional)
	tailer, err := natsio.NewNATSTailer(cfg.NATSURL, cfg.NATSAuthToken)
	if err != nil {
		slog.Warn("NATS tailer disabled or unavailable", "err", err)
	} else if tailer != nil {
		defer tailer.Close()
		slog.Info("connected to NATS JetStream for live log tailing", "url", cfg.NATSURL)
	}

	// 3. Instantiate QueryAPI Service
	querySvc := service.NewQueryAPI(chClient, tailer)

	// 4. Setup HTTP Handlers
	mux := http.NewServeMux()
	httpHandler := handler.NewHTTPHandler(querySvc)
	httpHandler.RegisterRoutes(mux)

	server := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second, // Allow extended SSE connections
		IdleTimeout:  60 * time.Second,
	}

	// 5. Graceful shutdown handler
	shutdownErr := make(chan error, 1)
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig

		slog.Info("shutting down Query API server...")
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()

		shutdownErr <- server.Shutdown(shutdownCtx)
	}()

	slog.Info("Query API service listening", "port", cfg.HTTPPort)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}

	return <-shutdownErr
}
