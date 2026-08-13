package agent

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewClientUsesIsolatedHTTPTransports(t *testing.T) {
	cfg := NewAgentConfig()
	cfg.AgentID = uuid.New()
	first := NewClient(cfg)
	second := NewClient(cfg)

	if first.httpClient == second.httpClient {
		t.Fatal("clients share an http.Client")
	}
	if first.httpClient.Transport == second.httpClient.Transport {
		t.Fatal("clients share an HTTP transport")
	}
}
