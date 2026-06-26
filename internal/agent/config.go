package agent

import (
	"time"

	"github.com/google/uuid"
)

var (
	DefaultAgentHeartbeatInterval = 5 * time.Second
	DefaultAgentMetricsInterval   = 30 * time.Second
)

type AgentConfig struct {
	ID         uuid.UUID
	AgentID    uuid.UUID
	Connection AgentConnectionConfig
	Heartbeat  AgentHeartbeatConfig
	Metrics    AgentMetricsConfig
	Version    int
}

type AgentConnectionConfig struct {
	Host   string
	Scheme string
	APIKey string
}

type AgentHeartbeatConfig struct {
	Interval time.Duration
}

type AgentMetricsConfig struct {
	Interval time.Duration
}

func NewAgentConfig() *AgentConfig {
	return &AgentConfig{
		Heartbeat: AgentHeartbeatConfig{
			Interval: DefaultAgentHeartbeatInterval,
		},
		Metrics: AgentMetricsConfig{
			Interval: DefaultAgentMetricsInterval,
		},
	}
}
