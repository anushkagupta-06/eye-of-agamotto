// Command writer consumes telemetry from NATS JetStream and bulk-inserts it into ClickHouse.
// Messages are acknowledged only after the insert succeeded (at-least-once, deduplicated).
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

	"github.com/anushkagupta-06/eye-of-agamotto/services/writer/internal/clickhouse"
	"github.com/anushkagupta-06/eye-of-agamotto/services/writer/internal/config"
	"github.com/anushkagupta-06/eye-of-agamotto/services/writer/internal/metrics"
	"github.com/anushkagupta-06/eye-of-agamotto/services/writer/internal/model"
	"github.com/anushkagupta-06/eye-of-agamotto/services/writer/internal/natsio"
	"github.com/anushkagupta-06/eye-of-agamotto/services/writer/internal/pipeline"
	"github.com/anushkagupta-06/eye-of-agamotto/services/writer/internal/transform"
)

const (
	subjectTraces  = "telemetry.traces"
	subjectLogs    = "telemetry.logs"
	durableTraces  = "writer-traces"
	durableLogs    = "writer-logs"
	signalTraces   = "traces"
	signalLogs     = "logs"
	startupTimeout = 20 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("writer exited with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	startCtx, cancelStart := context.WithTimeout(context.Background(), startupTimeout)
	defer cancelStart()

	sink, err := clickhouse.Open(startCtx, clickhouse.Config{
		Addr: cfg.ClickHouseAddr, Database: cfg.ClickHouseDatabase,
		User: cfg.ClickHouseUser, Password: cfg.ClickHousePassword,
		MaxConns: cfg.ClickHouseMaxConns, Timeout: cfg.InsertTimeout,
	})
	if err != nil {
		return err
	}
	defer sink.Close()

	nc, err := natsio.Connect(startCtx, natsio.Config{
		URL: cfg.NATSURL, Token: cfg.NATSToken, Stream: cfg.NATSStream,
		ConnectTimeout: 5 * time.Second, Log: log,
	})
	if err != nil {
		return err
	}
	for _, c := range []natsio.ConsumerConfig{
		{Durable: durableTraces, Subject: subjectTraces, AckWait: cfg.AckWait, MaxAckPending: cfg.MaxAckPending},
		{Durable: durableLogs, Subject: subjectLogs, AckWait: cfg.AckWait, MaxAckPending: cfg.MaxAckPending},
	} {
		if err := nc.EnsureConsumer(startCtx, c); err != nil {
			return err
		}
	}

	m := metrics.New()
	workerCfg := func(sig string) pipeline.Config {
		return pipeline.Config{
			Signal: sig, MaxMsgs: cfg.MaxMsgs, MaxWait: cfg.MaxWait, InsertTimeout: cfg.InsertTimeout,
			BackoffBase: cfg.BackoffBase, BackoffMax: cfg.BackoffMax,
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	err = spawn(ctx, &wg, cfg.Workers, func() (*pipeline.Worker[model.Span], error) {
		src, err := nc.Source(startCtx, durableTraces)
		if err != nil {
			return nil, err
		}
		return pipeline.NewWorker[model.Span](workerCfg(signalTraces), src, transform.DecodeSpans, sink.Spans(), nc.DeadLetters(), m, log)
	})
	if err != nil {
		return err
	}
	err = spawn(ctx, &wg, cfg.Workers, func() (*pipeline.Worker[model.Log], error) {
		src, err := nc.Source(startCtx, durableLogs)
		if err != nil {
			return nil, err
		}
		return pipeline.NewWorker[model.Log](workerCfg(signalLogs), src, transform.DecodeLogs, sink.Logs(), nc.DeadLetters(), m, log)
	})
	if err != nil {
		return err
	}

	draining := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-draining:
			http.Error(w, "shutting down", http.StatusServiceUnavailable)
			return
		default:
		}
		rctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := errors.Join(sink.Ping(rctx), nc.Ready(rctx)); err != nil {
			log.Warn("not ready", "err", err)
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("GET /metrics", m.Handler())
	ops := &http.Server{Addr: cfg.OpsAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	opsErr := make(chan error, 1)
	go func() { opsErr <- ops.ListenAndServe() }()
	log.Info("writer started", "workers_per_signal", cfg.Workers, "ops", cfg.OpsAddr)

	select {
	case <-ctx.Done():
		log.Info("shutdown requested")
	case err := <-opsErr:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Error("ops server stopped unexpectedly", "err", err)
		}
		stop()
	}

	close(draining)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(cfg.ShutdownTimeout):
		log.Warn("shutdown timed out; unacknowledged messages will be redelivered")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = ops.Shutdown(shutdownCtx)
	err = nc.Close()
	log.Info("writer stopped")
	return err
}

// spawn builds n workers up front, so configuration errors surface before any starts consuming,
// then runs each in its own goroutine until ctx is cancelled.
func spawn[T any](ctx context.Context, wg *sync.WaitGroup, n int, build func() (*pipeline.Worker[T], error)) error {
	workers := make([]*pipeline.Worker[T], 0, n)
	for i := 0; i < n; i++ {
		w, err := build()
		if err != nil {
			return err
		}
		workers = append(workers, w)
	}
	for _, w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.Run(ctx)
		}()
	}
	return nil
}
