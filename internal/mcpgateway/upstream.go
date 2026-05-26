package mcpgateway

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// upstream is one downstream MCP server connected by the gateway.
// (Naming: from the agent's point of view the agent is the client, the
// gateway is upstream; from the gateway's point of view the real MCP
// servers are downstream. We call them `upstream` inside the gateway to
// match how the MCP SDK names client/server pairs.)
type upstream struct {
	Spec    ServerSpec
	Session *mcp.ClientSession
}

func (u *upstream) Close() {
	if u == nil || u.Session == nil {
		return
	}
	_ = u.Session.Close()
}

// dial spawns or dials the downstream server described by spec, then runs
// the MCP initialize handshake. The returned ClientSession remains tied to
// ctx for its full lifetime: cancelling ctx terminates the session, and the
// SDK uses ctx for both the connect handshake and the long-lived JSON-RPC
// transport. Callers therefore pass a ctx whose lifetime matches the desired
// session lifetime; startup-timeout enforcement happens outside this call.
func dial(ctx context.Context, spec ServerSpec) (*mcp.ClientSession, error) {
	transport, err := buildTransport(spec)
	if err != nil {
		return nil, err
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "agx-mcp", Version: "v0"}, nil)
	sess, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", spec.Name, err)
	}
	return sess, nil
}

// dialWithStartupTimeout calls dial under a cancellable child ctx, returning
// an error if the handshake doesn't finish before the spec's startup timeout.
// The session, when returned, remains tied to parent (not to the timeout
// ctx), so subsequent calls aren't killed by a stale timeout.
func dialWithStartupTimeout(parent context.Context, spec ServerSpec) (*mcp.ClientSession, error) {
	timeout := time.Duration(spec.StartupTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = time.Duration(DefaultStartupTimeoutSec) * time.Second
	}
	type result struct {
		sess *mcp.ClientSession
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		sess, err := dial(parent, spec)
		ch <- result{sess, err}
	}()
	select {
	case r := <-ch:
		return r.sess, r.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("connect %s: startup timeout after %s", spec.Name, timeout)
	case <-parent.Done():
		return nil, parent.Err()
	}
}

func buildTransport(spec ServerSpec) (mcp.Transport, error) {
	switch spec.Transport {
	case TransportStdio:
		if strings.TrimSpace(spec.Command) == "" {
			return nil, fmt.Errorf("server %q: stdio requires command", spec.Name)
		}
		cmd := exec.Command(spec.Command, spec.Args...)
		cmd.Env = stdioEnv(spec)
		cmd.Stderr = os.Stderr
		return &mcp.CommandTransport{Command: cmd}, nil
	case TransportHTTP:
		if strings.TrimSpace(spec.URL) == "" {
			return nil, fmt.Errorf("server %q: http requires url", spec.Name)
		}
		if _, err := url.Parse(spec.URL); err != nil {
			return nil, fmt.Errorf("server %q: invalid url: %w", spec.Name, err)
		}
		t := &mcp.StreamableClientTransport{Endpoint: spec.URL}
		if len(spec.Headers) > 0 {
			t.HTTPClient = headerClient(spec.Headers)
		}
		return t, nil
	default:
		return nil, fmt.Errorf("server %q: unknown transport %q", spec.Name, spec.Transport)
	}
}

// ProbeResult is what `agx mcp test` reports for a single server.
type ProbeResult struct {
	Server  string        `json:"server"`
	Tools   []string      `json:"tools"`
	Prompts []string      `json:"prompts"`
	Latency time.Duration `json:"latency_ns"`
}

// Probe connects once, enumerates tools and prompts, then disconnects.
// Used by `agx mcp test` for ad-hoc health checks.
func Probe(ctx context.Context, spec ServerSpec) (*ProbeResult, error) {
	spec.normalize()
	start := time.Now()
	sess, err := dialWithStartupTimeout(ctx, spec)
	if err != nil {
		return nil, err
	}
	defer sess.Close()

	out := &ProbeResult{Server: spec.Name}
	if t, err := sess.ListTools(ctx, nil); err == nil {
		for _, x := range t.Tools {
			out.Tools = append(out.Tools, x.Name)
		}
	}
	if p, err := sess.ListPrompts(ctx, nil); err == nil {
		for _, x := range p.Prompts {
			out.Prompts = append(out.Prompts, x.Name)
		}
	}
	out.Latency = time.Since(start)
	return out, nil
}

func stdioEnv(spec ServerSpec) []string {
	base := os.Environ()
	keep := make(map[string]struct{}, len(spec.EnvPassthrough))
	for _, k := range spec.EnvPassthrough {
		keep[k] = struct{}{}
	}
	// EnvPassthrough is an allow-list filter only when explicitly set.
	// An empty passthrough means "inherit everything", matching the codex
	// `env_vars` default and what users intuitively expect from stdio
	// children.
	var env []string
	if len(keep) == 0 {
		env = append(env, base...)
	} else {
		for _, kv := range base {
			name, _, ok := strings.Cut(kv, "=")
			if !ok {
				continue
			}
			if _, take := keep[name]; take {
				env = append(env, kv)
			}
		}
	}
	for k, v := range spec.Env {
		env = append(env, k+"="+v)
	}
	return env
}
