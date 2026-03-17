package cli

import (
	"strings"

	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
)

type keysPasteItemView struct {
	ID       string             `json:"id"`
	Provider domainkey.Provider `json:"provider"`
	Profile  string             `json:"profile"`
	Name     string             `json:"name"`
	Active   bool               `json:"active"`
}

func splitLines(in string) []string {
	in = strings.ReplaceAll(in, "\r\n", "\n")
	in = strings.ReplaceAll(in, "\r", "\n")
	raw := strings.Split(in, "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func splitProviderHint(line string) (providerRaw string, rest string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", ""
	}

	if left, right, ok := strings.Cut(trimmed, ":"); ok {
		if _, ok := domainkey.ParseProvider(left); ok {
			return left, strings.TrimSpace(right)
		}
	}

	fields := strings.Fields(trimmed)
	if len(fields) >= 2 {
		if _, ok := domainkey.ParseProvider(fields[0]); ok {
			return fields[0], strings.TrimSpace(trimmed[len(fields[0]):])
		}
	}

	return "", trimmed
}
