package mcpgateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Gateway is the running daemon: a single upstream-facing mcp.Server backed
// by N downstream ClientSessions, plus an HTTP listener for upstream agents
// to dial in. Construct with New, then call Serve.
type Gateway struct {
	cfgPath string
	logger  *slog.Logger

	mu        sync.Mutex
	cfg       *Config
	server    *mcp.Server
	upstreams map[string]*upstream
	router    *nameRouter
	audit     *AuditLog
}

// New loads the config from disk (or returns an empty config if the file
// doesn't exist yet) and returns an unstarted Gateway.
func New(cfgPath string, logger *slog.Logger) (*Gateway, error) {
	if logger == nil {
		logger = slog.Default()
	}
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return nil, err
	}
	return &Gateway{
		cfgPath:   cfgPath,
		logger:    logger,
		cfg:       cfg,
		upstreams: map[string]*upstream{},
		router:    newNameRouter(),
	}, nil
}

// Serve runs the gateway until ctx is cancelled or the HTTP listener fails.
// Connections to downstream servers that fail at startup are logged but
// not fatal: the gateway continues serving whatever it could connect to,
// so a single broken upstream doesn't take down tool access for the rest.
func (g *Gateway) Serve(ctx context.Context) error {
	if err := g.start(ctx); err != nil {
		return err
	}
	defer g.shutdown()

	auth, err := g.authMiddleware()
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return g.server
	}, nil)
	mux.Handle("/mcp", auth(handler))
	mux.Handle("/mcp/", auth(handler))
	mux.HandleFunc("/health", g.handleHealth)

	listenAddr := g.cfg.Gateway.Listen
	if listenAddr == "" {
		listenAddr = DefaultListen
	}
	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", listenAddr, err)
	}

	g.logger.Info("mcp gateway listening",
		"addr", listenAddr,
		"servers", g.connectedCount(),
		"audit", g.audit.Path(),
	)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (g *Gateway) start(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	auditPath := g.cfg.Gateway.Audit.Path
	if g.cfg.Gateway.Audit.Enabled {
		if auditPath == "" {
			p, err := DefaultAuditPath()
			if err != nil {
				return err
			}
			auditPath = p
		}
		log, err := OpenAuditLog(auditPath)
		if err != nil {
			return err
		}
		g.audit = log
	}

	g.server = mcp.NewServer(&mcp.Implementation{
		Name:    "agx-mcp-gateway",
		Version: "v0",
	}, nil)

	for _, spec := range g.cfg.Servers {
		if !spec.IsEnabled() {
			g.logger.Info("upstream disabled, skipping", "name", spec.Name)
			continue
		}
		if err := g.connectLocked(ctx, spec); err != nil {
			g.logger.Warn("upstream connect failed", "name", spec.Name, "err", err)
			continue
		}
	}
	return nil
}

func (g *Gateway) connectLocked(ctx context.Context, spec ServerSpec) error {
	sess, err := dialWithStartupTimeout(ctx, spec)
	if err != nil {
		return err
	}
	ups := &upstream{Spec: spec, Session: sess}
	if err := registerUpstream(ctx, g.server, ups, g.router, g.audit); err != nil {
		_ = sess.Close()
		return err
	}
	g.upstreams[spec.Name] = ups
	g.logger.Info("upstream connected", "name", spec.Name, "transport", spec.Transport)
	return nil
}

func (g *Gateway) shutdown() {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, u := range g.upstreams {
		u.Close()
	}
	if g.audit != nil {
		_ = g.audit.Close()
	}
}

func (g *Gateway) connectedCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.upstreams)
}

// ConnectedServers returns the names of upstream servers currently
// connected. Used by `agx mcp list` and health checks.
func (g *Gateway) ConnectedServers() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	names := make([]string, 0, len(g.upstreams))
	for n := range g.upstreams {
		names = append(names, n)
	}
	return names
}

func (g *Gateway) authMiddleware() (func(http.Handler) http.Handler, error) {
	envName := strings.TrimSpace(g.cfg.Gateway.Auth.BearerTokenEnv)
	if envName == "" {
		return func(h http.Handler) http.Handler { return h }, nil
	}
	token := strings.TrimSpace(os.Getenv(envName))
	if token == "" {
		return nil, fmt.Errorf("auth.bearer_token_env=%s set but env var is empty", envName)
	}
	expected := "Bearer " + token
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != expected {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			h.ServeHTTP(w, r)
		})
	}, nil
}

func (g *Gateway) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","servers":%d}`, g.connectedCount())
}
