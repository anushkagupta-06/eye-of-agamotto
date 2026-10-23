package transport

import (
	"context"
	"log/slog"
	"net"
	"strings"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/encoding/gzip" // registers the gzip compressor
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"

	"github.com/anushkagupta-06/eye-of-agamotto/services/ingestion-gateway/internal/ingest"
)

// GRPCConfig configures the OTLP/gRPC server.
type GRPCConfig struct {
	MaxRecvBytes int           // per-message limit
	RetryAfter   time.Duration // RetryInfo delay attached to retryable errors
}

// GRPCServer wraps a grpc.Server with the OTLP services, auth and health checking.
type GRPCServer struct {
	srv    *grpc.Server
	health *health.Server
}

type traceService struct {
	coltracepb.UnimplementedTraceServiceServer
	svc *ingest.Service
	cfg GRPCConfig
	log *slog.Logger
}

func (s *traceService) Export(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) (*coltracepb.ExportTraceServiceResponse, error) {
	resp, err := s.svc.Traces(ctx, TenantFrom(ctx), req)
	if err != nil {
		return nil, grpcError(s.log, err, s.cfg.RetryAfter)
	}
	return resp, nil
}

type logsService struct {
	collogspb.UnimplementedLogsServiceServer
	svc *ingest.Service
	cfg GRPCConfig
	log *slog.Logger
}

func (s *logsService) Export(ctx context.Context, req *collogspb.ExportLogsServiceRequest) (*collogspb.ExportLogsServiceResponse, error) {
	resp, err := s.svc.Logs(ctx, TenantFrom(ctx), req)
	if err != nil {
		return nil, grpcError(s.log, err, s.cfg.RetryAfter)
	}
	return resp, nil
}

// grpcError converts err to a status. Retryable conditions carry RetryInfo, which the OTLP
// specification requires for RESOURCE_EXHAUSTED to be retried by SDKs.
func grpcError(log *slog.Logger, err error, retryAfter time.Duration) error {
	f := classify(err)
	if f.log {
		log.Error("ingest failed", "err", err)
	}
	st := status.New(f.code, f.message)
	if f.retry {
		if withInfo, derr := st.WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(retryAfter)}); derr == nil {
			st = withInfo
		}
	}
	return st.Err()
}

// NewGRPCServer builds the server. Interceptor order: recover, metrics, auth+admission.
func NewGRPCServer(a Authenticator, svc *ingest.Service, cfg GRPCConfig, rec RequestRecorder, log *slog.Logger) *GRPCServer {
	srv := grpc.NewServer(
		grpc.MaxRecvMsgSize(cfg.MaxRecvBytes),
		grpc.MaxConcurrentStreams(1024),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 5 * time.Minute,
			Time:              2 * time.Minute,
			Timeout:           20 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             30 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.ChainUnaryInterceptor(
			recoverInterceptor(log),
			metricsInterceptor(rec),
			authInterceptor(a, svc, cfg, log),
		),
	)
	coltracepb.RegisterTraceServiceServer(srv, &traceService{svc: svc, cfg: cfg, log: log})
	collogspb.RegisterLogsServiceServer(srv, &logsService{svc: svc, cfg: cfg, log: log})

	hs := health.NewServer()
	healthpb.RegisterHealthServer(srv, hs)
	return &GRPCServer{srv: srv, health: hs}
}

// Serve blocks until the listener is closed or the server stops.
func (g *GRPCServer) Serve(l net.Listener) error { return g.srv.Serve(l) }

// Shutdown reports NOT_SERVING, drains in-flight RPCs, and force-stops when ctx expires.
func (g *GRPCServer) Shutdown(ctx context.Context) {
	g.health.Shutdown()
	done := make(chan struct{})
	go func() { g.srv.GracefulStop(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		g.srv.Stop()
	}
}

func isHealth(method string) bool { return strings.HasPrefix(method, "/grpc.health.v1.Health/") }

func signalOf(method string) string {
	switch {
	case strings.Contains(method, "TraceService"):
		return "traces"
	case strings.Contains(method, "LogsService"):
		return "logs"
	default:
		return "other"
	}
}

func recoverInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("panic in grpc handler", "panic", r, "method", info.FullMethod)
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return h(ctx, req)
	}
}

func metricsInterceptor(rec RequestRecorder) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		resp, err := h(ctx, req)
		if !isHealth(info.FullMethod) {
			rec.Request("grpc", signalOf(info.FullMethod), int(status.Code(err)))
		}
		return resp, err
	}
}

func firstValue(md metadata.MD, key string) string {
	if v := md.Get(key); len(v) > 0 {
		return v[0]
	}
	return ""
}

func authInterceptor(a Authenticator, svc *ingest.Service, cfg GRPCConfig, log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		if isHealth(info.FullMethod) {
			return h(ctx, req)
		}
		md, _ := metadata.FromIncomingContext(ctx)
		key := firstValue(md, "x-api-key")
		if key == "" {
			key = bearer(firstValue(md, "authorization"))
		}
		tenant, err := a.Authenticate(key)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid or missing API key")
		}
		if err := svc.Admit(tenant); err != nil {
			return nil, grpcError(log, err, cfg.RetryAfter)
		}
		return h(WithTenant(ctx, tenant), req)
	}
}
