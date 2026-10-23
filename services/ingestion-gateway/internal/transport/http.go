package transport

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"

	"github.com/anushkagupta-06/eye-of-agamotto/services/ingestion-gateway/internal/ingest"
	"github.com/anushkagupta-06/eye-of-agamotto/services/ingestion-gateway/internal/otlpjson"
)

// HTTPConfig configures the OTLP/HTTP handler.
type HTTPConfig struct {
	MaxRequestBytes int64         // limit on the decompressed body
	RetryAfter      time.Duration // Retry-After hint for 429/503
}

type signal struct {
	name   string
	newReq func() proto.Message
	export func(ctx context.Context, tenant string, req proto.Message) (proto.Message, error)
}

type httpHandler struct {
	auth Authenticator
	svc  *ingest.Service
	cfg  HTTPConfig
	rec  RequestRecorder
	log  *slog.Logger
}

// NewHTTPHandler serves POST /v1/traces and POST /v1/logs per the OTLP/HTTP specification
// (protobuf or JSON bodies, optional gzip).
func NewHTTPHandler(a Authenticator, svc *ingest.Service, cfg HTTPConfig, rec RequestRecorder, log *slog.Logger) http.Handler {
	h := &httpHandler{auth: a, svc: svc, cfg: cfg, rec: rec, log: log}

	traces := signal{
		name:   "traces",
		newReq: func() proto.Message { return new(coltracepb.ExportTraceServiceRequest) },
		export: func(ctx context.Context, tenant string, req proto.Message) (proto.Message, error) {
			resp, err := svc.Traces(ctx, tenant, req.(*coltracepb.ExportTraceServiceRequest))
			if err != nil {
				return nil, err
			}
			return resp, nil
		},
	}
	logs := signal{
		name:   "logs",
		newReq: func() proto.Message { return new(collogspb.ExportLogsServiceRequest) },
		export: func(ctx context.Context, tenant string, req proto.Message) (proto.Message, error) {
			resp, err := svc.Logs(ctx, tenant, req.(*collogspb.ExportLogsServiceRequest))
			if err != nil {
				return nil, err
			}
			return resp, nil
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/traces", h.serve(traces))
	mux.HandleFunc("POST /v1/logs", h.serve(logs))
	return recoverMiddleware(log, mux)
}

func (h *httpHandler) serve(sig signal) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := h.handle(w, r, sig)
		h.rec.Request("http", sig.name, code)
	}
}

func apiKeyFromHTTP(r *http.Request) string {
	if k := r.Header.Get("X-API-Key"); k != "" {
		return k
	}
	return bearer(r.Header.Get("Authorization"))
}

func (h *httpHandler) handle(w http.ResponseWriter, r *http.Request, sig signal) int {
	tenant, err := h.auth.Authenticate(apiKeyFromHTTP(r))
	if err != nil {
		w.Header().Set("WWW-Authenticate", "Bearer")
		return h.fail(w, http.StatusUnauthorized, codes.Unauthenticated, "invalid or missing API key", false, false)
	}
	if err := h.svc.Admit(tenant); err != nil {
		return h.failErr(w, err, false)
	}

	asJSON := false
	switch ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); ct {
	case "application/x-protobuf":
	case "application/json":
		asJSON = true
	default:
		return h.fail(w, http.StatusUnsupportedMediaType, codes.InvalidArgument,
			"Content-Type must be application/x-protobuf or application/json", false, false)
	}

	body := io.Reader(http.MaxBytesReader(w, r.Body, h.cfg.MaxRequestBytes))
	switch strings.ToLower(r.Header.Get("Content-Encoding")) {
	case "", "identity":
	case "gzip":
		zr, err := gzip.NewReader(body)
		if err != nil {
			return h.fail(w, http.StatusBadRequest, codes.InvalidArgument, "invalid gzip body", false, asJSON)
		}
		defer zr.Close()
		// Bound the decompressed size too, so a small gzip bomb cannot exhaust memory.
		body = io.LimitReader(zr, h.cfg.MaxRequestBytes+1)
	default:
		return h.fail(w, http.StatusUnsupportedMediaType, codes.InvalidArgument,
			"Content-Encoding must be identity or gzip", false, asJSON)
	}

	data, err := io.ReadAll(body)
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) || int64(len(data)) > h.cfg.MaxRequestBytes {
		return h.fail(w, http.StatusRequestEntityTooLarge, codes.InvalidArgument, "request too large", false, asJSON)
	}
	if err != nil {
		return h.fail(w, http.StatusBadRequest, codes.InvalidArgument, "could not read request body", false, asJSON)
	}

	req := sig.newReq()
	if asJSON {
		// OTLP/JSON carries ids as hex, protojson expects base64: convert first.
		var normalized []byte
		if normalized, err = otlpjson.HexIDsToBase64(data); err == nil {
			err = protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(normalized, req)
		}
	} else {
		err = proto.Unmarshal(data, req)
	}
	if err != nil {
		return h.fail(w, http.StatusBadRequest, codes.InvalidArgument, "malformed OTLP payload", false, asJSON)
	}

	resp, err := sig.export(r.Context(), tenant, req)
	if err != nil {
		return h.failErr(w, err, asJSON)
	}

	var out []byte
	ct := "application/x-protobuf"
	if asJSON {
		ct = "application/json"
		out, err = protojson.Marshal(resp)
	} else {
		out, err = proto.Marshal(resp)
	}
	if err != nil {
		return h.fail(w, http.StatusInternalServerError, codes.Internal, "internal error", false, asJSON)
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
	return http.StatusOK
}

func (h *httpHandler) failErr(w http.ResponseWriter, err error, asJSON bool) int {
	f := classify(err)
	if f.log {
		h.log.Error("ingest failed", "err", err)
	} else if f.httpStatus != http.StatusTooManyRequests && f.code != codes.Canceled {
		h.log.Warn("ingest rejected", "err", err)
	}
	return h.fail(w, f.httpStatus, f.code, f.message, f.retry, asJSON)
}

// fail writes a google.rpc.Status body as required by the OTLP/HTTP specification.
func (h *httpHandler) fail(w http.ResponseWriter, httpStatus int, code codes.Code, msg string, retry, asJSON bool) int {
	if retry {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds(h.cfg.RetryAfter)))
	}
	st := status.New(code, msg).Proto()
	var (
		body []byte
		ct   = "application/x-protobuf"
	)
	if asJSON {
		ct = "application/json"
		body, _ = protojson.Marshal(st)
	} else {
		body, _ = proto.Marshal(st)
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(httpStatus)
	_, _ = w.Write(body)
	return httpStatus
}

func recoverMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("panic in http handler", "panic", rec, "path", r.URL.Path)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// NewHTTPServer applies conservative timeouts to the OTLP/HTTP listener.
func NewHTTPServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
}