// Command gateway is the OTLP ingestion gateway: it authenticates and validates telemetry,
// batches it, and acknowledges clients only after it is durable in NATS JetStream.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/anushkagupta-06/eye-of-agamotto/services/ingestion-gateway/internal/auth"
	"github.com/anushkagupta-06/eye-of-agamotto/services/ingestion-gateway/internal/batcher"
	"github.com/anushkagupta-06/eye-of-agamotto/services/ingestion-gateway/internal/config"
	"github.com/anushkagupta-06/eye-of-agamotto/services/ingestion-gateway/internal/ingest"
	"github.com/anushkagupta-06/eye-of-agamotto/services/ingestion-gateway/internal/metrics"
	"github.com/anushkagupta-06/eye-of-agamotto/services/ingestion-gateway/internal/publisher"
	"github.com/anushkagupta-06/eye-of-agamotto/services/ingestion-gateway/internal/ratelimit"
	"github.com/anushkagupta-06/eye-of-agamotto/services/ingestion-gateway/internal/transport"
)

const (
	subjectTraces = "telemetry.traces"
	subjectLogs   = "telemetry.logs"
)

func main() {
	if err := run(); err != nil {
		slog.Error("gateway exited with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	keys, err := auth.Load(cfg.APIKeysFile)
	if err != nil {
		return err
	}
	log.Info("api keys loaded", "count", keys.Len())

	startCtx, cancelStart := context.WithTimeout(context.Background(), 15*time.Second)
	pub, err := publisher.Connect(startCtx, publisher.Config{
		URL: cfg.NATSURL, Token: cfg.NATSToken, Stream: cfg.NATSStream,
		ConnectTimeout: 5 * time.Second, Log: log,
	})
	cancelStart()
	if err != nil {
		return err
	}

	m := metrics.New()
	newBatcher := func(subject string) (*batcher.Batcher, error) {
		b, err := batcher.New(batcher.Config{
			Subject:          subject,
			TargetBytes:      cfg.BatchTargetBytes,
			MaxBytes:         cfg.BatchMaxBytes,
			MaxDelay:         cfg.BatchMaxDelay,
			MaxInflightBytes: cfg.MaxInflightBytes,
			PublishTimeout:   cfg.PublishTimeout,
			Observer:         m.BatchObserver(),
		}, pub)
		if err == nil {
			m.Inflight(subject, b.Inflight)
		}
		return b, err
	}
	tracesB, err := newBatcher(subjectTraces)
	if err != nil {
		return err
	}
	logsB, err := newBatcher(subjectLogs)
	if err != nil {
		return err
	}

	svc := ingest.New(tracesB, logsB, ratelimit.New(cfg.RateLimitRPS, cfg.RateLimitBurst), m)

	grpcSrv := transport.NewGRPCServer(keys, svc, transport.GRPCConfig{
		MaxRecvBytes: int(cfg.MaxRequestBytes), RetryAfter: cfg.RetryAfter,
	}, m, log)
	httpSrv := transport.NewHTTPServer(cfg.HTTPAddr, transport.NewHTTPHandler(keys, svc, transport.HTTPConfig{
		MaxRequestBytes: cfg.MaxRequestBytes, RetryAfter: cfg.RetryAfter,
	}, m, log))

	var shuttingDown sync.Once
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
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pub.Ready(ctx); err != nil {
			log.Warn("not ready", "err", err)
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("GET /metrics", m.Handler())
	opsSrv := &http.Server{Addr: cfg.OpsAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	grpcLn, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("listen grpc: %w", err)
	}

	errc := make(chan error, 3)
	go func() { errc <- fmt.Errorf("grpc server: %w", grpcSrv.Serve(grpcLn)) }()
	go func() { errc <- fmt.Errorf("http server: %w", httpSrv.ListenAndServe()) }()
	go func() { errc <- fmt.Errorf("ops server: %w", opsSrv.ListenAndServe()) }()
	log.Info("gateway started", "grpc", cfg.GRPCAddr, "http", cfg.HTTPAddr, "ops", cfg.OpsAddr)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	for {
		select {
		case s := <-sigs:
			if s == syscall.SIGHUP { // rotate API keys without a restart
				if err := keys.Reload(); err != nil {
					log.Error("api key reload failed; keeping previous keys", "err", err)
				} else {
					log.Info("api keys reloaded", "count", keys.Len())
				}
				continue
			}
			log.Info("shutdown requested", "signal", s.String())
		case err := <-errc:
			if !errors.Is(err, http.ErrServerClosed) {
				log.Error("server stopped unexpectedly", "err", err)
			}
		}
		break
	}

	var shutdownErr error
	shuttingDown.Do(func() {
		close(draining) // readiness flips first so load balancers stop sending traffic
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		grpcSrv.Shutdown(ctx)
		_ = httpSrv.Shutdown(ctx)
		// Servers are drained, so no new Submit calls arrive; flush what is batched.
		for name, b := range map[string]*batcher.Batcher{subjectTraces: tracesB, subjectLogs: logsB} {
			if err := b.Close(ctx); err != nil {
				shutdownErr = errors.Join(shutdownErr, fmt.Errorf("flush %s: %w", name, err))
			}
		}
		if err := pub.Close(); err != nil {
			shutdownErr = errors.Join(shutdownErr, err)
		}
		_ = opsSrv.Shutdown(ctx)
	})
	log.Info("gateway stopped")
	return shutdownErr
}
