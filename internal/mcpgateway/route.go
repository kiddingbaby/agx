package mcpgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const NamespaceSep = "__"

// nameRouter remembers the (server,original) pair for every qualified
// name we expose on the upstream so reverse lookup is O(1) even when the
// original name itself contains the separator.
type nameRouter struct {
	mu    sync.RWMutex
	tools map[string]toolRoute // key: qualified name
}

type toolRoute struct {
	Server string
	Origin string
}

func newNameRouter() *nameRouter {
	return &nameRouter{tools: map[string]toolRoute{}}
}

func (n *nameRouter) putTool(qualified, server, origin string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.tools[qualified] = toolRoute{Server: server, Origin: origin}
}

func (n *nameRouter) lookupTool(qualified string) (toolRoute, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	r, ok := n.tools[qualified]
	return r, ok
}

func (n *nameRouter) dropServer(server string) []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	var dropped []string
	for q, r := range n.tools {
		if r.Server == server {
			dropped = append(dropped, q)
			delete(n.tools, q)
		}
	}
	return dropped
}

func qualifiedName(server, name string) string {
	return server + NamespaceSep + name
}

// registerUpstream pulls tools and prompts from a downstream session and
// adds them to the gateway's upstream-facing mcp.Server with namespaced
// names. Returns the list of qualified tool names registered so callers can
// remove them on reload.
//
// Resources are intentionally not forwarded in v1: their URIs don't admit
// a clean namespacing scheme without a bespoke proxy URI format, and the
// five tracked servers don't expose resources today. (See gateway TODO.)
func registerUpstream(ctx context.Context, srv *mcp.Server, ups *upstream, router *nameRouter, audit *AuditLog) error {
	if err := registerTools(ctx, srv, ups, router, audit); err != nil {
		return err
	}
	if err := registerPrompts(ctx, srv, ups); err != nil {
		return err
	}
	return nil
}

func registerTools(ctx context.Context, srv *mcp.Server, ups *upstream, router *nameRouter, audit *AuditLog) error {
	res, err := ups.Session.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("%s tools/list: %w", ups.Spec.Name, err)
	}
	for _, t := range res.Tools {
		qualified := qualifiedName(ups.Spec.Name, t.Name)
		tcopy := *t
		tcopy.Name = qualified
		router.putTool(qualified, ups.Spec.Name, t.Name)
		srv.AddTool(&tcopy, makeToolHandler(ups, t.Name, audit))
	}
	return nil
}

func makeToolHandler(ups *upstream, originalName string, audit *AuditLog) mcp.ToolHandler {
	server := ups.Spec.Name
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params := &mcp.CallToolParams{
			Name: originalName,
		}
		if len(req.Params.Arguments) > 0 {
			params.Arguments = json.RawMessage(req.Params.Arguments)
		}
		params.Meta = req.Params.Meta
		start := time.Now()
		result, err := ups.Session.CallTool(ctx, params)
		ev := AuditEvent{
			Timestamp: start.UTC(),
			Server:    server,
			Tool:      originalName,
			Duration:  time.Since(start),
		}
		switch {
		case err != nil:
			ev.Status = "error"
			ev.Error = err.Error()
		case result != nil && result.IsError:
			ev.Status = "tool_error"
		default:
			ev.Status = "ok"
		}
		audit.Record(ev)
		return result, err
	}
}

func registerPrompts(ctx context.Context, srv *mcp.Server, ups *upstream) error {
	res, err := ups.Session.ListPrompts(ctx, nil)
	if err != nil {
		// Plenty of servers don't implement prompts, and the SDK surfaces
		// the resulting error in a way that's hard to distinguish from a
		// real failure. Treat any error here as "no prompts" rather than
		// killing the upstream registration; we'd rather lose prompt
		// forwarding for that server than lose all its tools.
		return nil
	}
	for _, p := range res.Prompts {
		pcopy := *p
		pcopy.Name = qualifiedName(ups.Spec.Name, p.Name)
		srv.AddPrompt(&pcopy, makePromptHandler(ups, p.Name))
	}
	return nil
}

func makePromptHandler(ups *upstream, originalName string) mcp.PromptHandler {
	return func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		params := &mcp.GetPromptParams{
			Name:      originalName,
			Arguments: req.Params.Arguments,
			Meta:      req.Params.Meta,
		}
		return ups.Session.GetPrompt(ctx, params)
	}
}
