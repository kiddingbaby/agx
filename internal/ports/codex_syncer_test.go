package ports

import (
	"strings"
	"testing"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

func TestIncompleteManagedBlockErrorMessages(t *testing.T) {
	cases := []struct {
		name string
		err  *IncompleteManagedBlockError
		want string
	}{
		{
			name: "agent and path",
			err:  &IncompleteManagedBlockError{Agent: domainprofile.AgentCodex, ConfigPath: "/tmp/config.toml"},
			want: "codex config has an incomplete AGX managed block: /tmp/config.toml",
		},
		{
			name: "agent only",
			err:  &IncompleteManagedBlockError{Agent: domainprofile.AgentGemini},
			want: "gemini config has an incomplete AGX managed block",
		},
		{
			name: "generic",
			err:  &IncompleteManagedBlockError{},
			want: "config has an incomplete AGX managed block",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); !strings.Contains(got, tc.want) {
				t.Fatalf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}
