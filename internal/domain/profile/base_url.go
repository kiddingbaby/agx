package profile

import (
	"net/url"
	"strings"
)

func AgentBaseURL(agent Agent, raw string) string {
	switch agent {
	case AgentCodex:
		return BaseURLWithV1(raw)
	case AgentClaude, AgentGemini:
		return BaseURLWithoutTrailingV1(raw)
	case AgentOpenCode:
		return NormalizeBaseURL(raw)
	default:
		return NormalizeBaseURL(raw)
	}
}

func BaseURLWithV1(raw string) string {
	normalized := NormalizeBaseURL(raw)
	if normalized == "" {
		return ""
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return normalized
	}

	path := strings.TrimRight(parsed.Path, "/")
	switch {
	case path == "":
		parsed.Path = "/v1"
	case strings.HasSuffix(path, "/v1"):
		parsed.Path = path
	default:
		parsed.Path = path + "/v1"
	}
	return parsed.String()
}

func BaseURLWithoutTrailingV1(raw string) string {
	normalized := NormalizeBaseURL(raw)
	if normalized == "" {
		return ""
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return normalized
	}

	path := strings.TrimRight(parsed.Path, "/")
	if path == "/v1" {
		parsed.Path = ""
		return parsed.String()
	}
	if strings.HasSuffix(path, "/v1") {
		parsed.Path = strings.TrimSuffix(path, "/v1")
	}
	return parsed.String()
}
