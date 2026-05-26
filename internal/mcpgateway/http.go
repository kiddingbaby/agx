package mcpgateway

import (
	"net/http"
)

// headerClient returns an *http.Client whose transport injects the given
// headers into every request. Used to attach Authorization / custom headers
// to upstream HTTP MCP servers.
func headerClient(headers map[string]string) *http.Client {
	return &http.Client{Transport: &headerRT{base: http.DefaultTransport, headers: headers}}
}

type headerRT struct {
	base    http.RoundTripper
	headers map[string]string
}

func (h *headerRT) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(h.headers) > 0 {
		clone := req.Clone(req.Context())
		for k, v := range h.headers {
			clone.Header.Set(k, v)
		}
		req = clone
	}
	return h.base.RoundTrip(req)
}
