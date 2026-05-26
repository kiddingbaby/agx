package cli

import "testing"

func TestNormalizeListenForClient(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "127.0.0.1:8765"},
		{"127.0.0.1:8765", "127.0.0.1:8765"},
		{":8765", "127.0.0.1:8765"},
		{"0.0.0.0:9000", "127.0.0.1:9000"},
		{"[::]:9000", "127.0.0.1:9000"},
		{"mcp.example.com:80", "mcp.example.com:80"},
	}
	for _, c := range cases {
		if got := normalizeListenForClient(c.in); got != c.want {
			t.Errorf("normalizeListenForClient(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
