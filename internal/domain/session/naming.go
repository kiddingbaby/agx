package session

import "strings"

const sessionPrefix = "ai-"

func SessionName(agentName string) string {
	return NormalizeSessionName(agentName)
}

func NormalizeSessionName(name string) string {
	if strings.HasPrefix(name, sessionPrefix) {
		return name
	}
	return sessionPrefix + name
}

func IsManagedSessionName(name string) bool {
	return strings.HasPrefix(name, sessionPrefix)
}
