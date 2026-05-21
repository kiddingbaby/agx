package profile

import (
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestPropertyNormalizeProfileNameAlwaysLowerTrimmed(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		raw := rapid.String().Draw(t, "raw")
		got := NormalizeProfileName(raw)

		if got != strings.ToLower(strings.TrimSpace(raw)) {
			t.Fatalf("NormalizeProfileName(%q) = %q", raw, got)
		}
		if got != strings.TrimSpace(got) {
			t.Fatalf("normalized name still has leading or trailing spaces: %q", got)
		}
	})
}

func TestPropertyNormalizeBaseURLIdempotent(t *testing.T) {
	validishURL := rapid.Custom(func(t *rapid.T) string {
		scheme := rapid.SampledFrom([]string{"http", "https", "HTTP", "HTTPS"}).Draw(t, "scheme")
		host := rapid.StringMatching(`[A-Za-z0-9.-]{1,20}`).Draw(t, "host")
		path := rapid.StringMatching(`(/[A-Za-z0-9._-]{0,12}){0,3}/?`).Draw(t, "path")
		return scheme + "://" + host + path
	})

	rapid.Check(t, func(t *rapid.T) {
		raw := validishURL.Draw(t, "raw")
		once := NormalizeBaseURL(raw)
		twice := NormalizeBaseURL(once)

		if once != twice {
			t.Fatalf("NormalizeBaseURL is not idempotent: raw=%q once=%q twice=%q", raw, once, twice)
		}
		if strings.Contains(once, "#") {
			t.Fatalf("normalized url still contains fragment: %q", once)
		}
	})
}

func TestPropertyParseAgentRoundTripsSupportedAgents(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		agent := rapid.SampledFrom(SupportedAgents()).Draw(t, "agent")
		padding := rapid.SampledFrom([]string{"", " ", "  ", "\t"}).Draw(t, "padding")
		raw := padding + strings.ToUpper(string(agent)) + padding

		got, ok := ParseAgent(raw)
		if !ok {
			t.Fatalf("ParseAgent(%q) reported !ok", raw)
		}
		if got != agent {
			t.Fatalf("ParseAgent(%q) = %q, want %q", raw, got, agent)
		}
	})
}
