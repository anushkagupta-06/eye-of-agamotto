package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anushkagupta-06/eye-of-agamotto/services/mcp-server/internal/client"
	"github.com/anushkagupta-06/eye-of-agamotto/services/mcp-server/internal/config"
	"github.com/anushkagupta-06/eye-of-agamotto/services/mcp-server/internal/server"
)

func main() {
	stdioFlag := flag.Bool("stdio", false, "Run in stdio mode for desktop AI clients (Claude Desktop, Cursor, Antigravity)")
	portFlag := flag.String("port", "", "HTTP port to listen on (e.g. 8083)")
	queryAPIURL := flag.String("query-api", "", "URL to QueryAPI service (e.g. http://127.0.0.1:8082)")
	tenantFlag := flag.String("tenant", "", "Default tenant ID (default demo)")
	flag.Parse()

	cfg := config.Load()
	if *stdioFlag {
		cfg.Mode = "stdio"
	}
	if *portFlag != "" {
		cfg.HTTPPort = *portFlag
	}
	if *queryAPIURL != "" {
		cfg.QueryAPIURL = *queryAPIURL
	}
	if *tenantFlag != "" {
		cfg.DefaultTenant = *tenantFlag
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Initialize QueryAPI delegate client
	queryClient := client.NewQueryAPIClient(cfg.QueryAPIURL, cfg.DefaultTenant)

	// Initialize MCP handler with the 6 tools
	mcpHandler := server.NewMCPHandler(queryClient)

	if cfg.Mode == "stdio" {
		// Stdio mode: no log output to stdout so we don't corrupt JSON-RPC stream
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
		slog.Info("starting Eye of Agamotto MCP server in stdio mode", "queryAPI", cfg.QueryAPIURL)
		if err := server.ServeStdio(ctx, mcpHandler); err != nil {
			slog.Error("stdio server exited with error", "err", err)
			os.Exit(1)
		}
		return
	}

	// HTTP / SSE mode
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	mux := http.NewServeMux()
	httpServer := server.NewHTTPServer(mcpHandler)
	httpServer.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		slog.Info("shutting down MCP HTTP server...")
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelShutdown()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("Eye of Agamotto MCP server listening", "port", cfg.HTTPPort, "delegatingTo", cfg.QueryAPIURL)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("mcp http server fatal error", "err", err)
		os.Exit(1)
	}
}
