package mcpgateway

import (
	"path/filepath"
	"testing"
)

func TestLoadConfigMissingReturnsEmpty(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Servers) != 0 {
		t.Errorf("expected empty, got %d", len(cfg.Servers))
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "servers.yaml")
	disabled := false
	in := &Config{
		Servers: []ServerSpec{
			{Name: "stdio-svc", Command: "node", Args: []string{"a.js"}},
			{Name: "http-svc", URL: "https://x/mcp", Enabled: &disabled},
		},
		Gateway: GatewaySpec{Listen: "127.0.0.1:9000"},
	}
	if err := SaveConfig(path, in); err != nil {
		t.Fatal(err)
	}
	out, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Servers) != 2 {
		t.Fatalf("want 2 servers, got %d", len(out.Servers))
	}
	if out.Servers[0].Transport != TransportStdio {
		t.Errorf("first server transport not inferred to stdio: %s", out.Servers[0].Transport)
	}
	if out.Servers[1].Transport != TransportHTTP {
		t.Errorf("second server transport not inferred to http: %s", out.Servers[1].Transport)
	}
	if out.Servers[1].IsEnabled() {
		t.Errorf("second server should be disabled")
	}
	if out.Gateway.Listen != "127.0.0.1:9000" {
		t.Errorf("listen lost: %s", out.Gateway.Listen)
	}
}

func TestValidateDetectsDuplicates(t *testing.T) {
	cfg := &Config{Servers: []ServerSpec{
		{Name: "x", URL: "http://e", Transport: TransportHTTP},
		{Name: "x", URL: "http://e2", Transport: TransportHTTP},
	}}
	if err := cfg.Validate(); err == nil {
		t.Errorf("expected duplicate error")
	}
}

func TestValidateStdioRequiresCommand(t *testing.T) {
	cfg := &Config{Servers: []ServerSpec{{Name: "x", Transport: TransportStdio}}}
	if err := cfg.Validate(); err == nil {
		t.Errorf("expected missing-command error")
	}
}

func TestUpsertServerReplaces(t *testing.T) {
	cfg := &Config{}
	cfg.UpsertServer(ServerSpec{Name: "x", URL: "http://e"})
	if replaced := cfg.UpsertServer(ServerSpec{Name: "x", URL: "http://e2"}); !replaced {
		t.Errorf("second upsert should report replacement")
	}
	if len(cfg.Servers) != 1 || cfg.Servers[0].URL != "http://e2" {
		t.Errorf("upsert did not replace: %+v", cfg.Servers)
	}
}

func TestSetEnabledMissingServerReturnsFalse(t *testing.T) {
	cfg := &Config{}
	if cfg.SetEnabled("nope", true) {
		t.Errorf("expected false for missing server")
	}
}
