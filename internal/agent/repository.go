package agent

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type CreateAgentDTO struct {
	HostID  uuid.UUID
	Version string
}

type CreateAgentConfigDTO struct {
	AgentID   uuid.UUID
	Heartbeat AgentHeartbeatConfig
	Metrics   AgentMetricsConfig
	Version   int
}

type Repository interface {
	Create(ctx context.Context, data CreateAgentDTO) (Agent, error)
	List(ctx context.Context) ([]Agent, error)
	GetByID(ctx context.Context, id uuid.UUID) (Agent, error)
	GetByHostID(ctx context.Context, hostID uuid.UUID) (Agent, error)
	Update(ctx context.Context, agent *Agent) error
	Delete(ctx context.Context, id uuid.UUID) error
	UpdateLastSeenAt(ctx context.Context, id uuid.UUID, lastSeenAt time.Time) error
}

type AgentConfigRepository interface {
	Create(ctx context.Context, data CreateAgentConfigDTO) (AgentConfig, error)
	GetByAgentID(ctx context.Context, agentID uuid.UUID) (AgentConfig, error)
	Update(ctx context.Context, agentConfig *AgentConfig) error
}
