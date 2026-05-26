// Package mcpgateway is agx's built-in MCP gateway: it fans out a single
// upstream MCP endpoint to multiple downstream MCP servers (stdio or HTTP),
// records tool calls to a local audit log, and exposes per-server enable /
// disable so the same set of tools serves every agent (codex, claude,
// gemini, opencode) through one HTTP endpoint instead of N per-agent configs.
//
// The gateway is a daemon: users run `agx mcp serve` (typically under
// systemd / launchd / nohup) and four-agent configs only need a single line
// pointing at it. agx itself does not manage the daemon lifecycle.
package mcpgateway
