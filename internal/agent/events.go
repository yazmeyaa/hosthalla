package agent

import "github.com/google/uuid"

type AgentRegisteredEvent struct {
	AgentID uuid.UUID
	HostID  uuid.UUID
}

func (e AgentRegisteredEvent) EventName() string {
	return "agent.registered"
}

type AgentUpdatedEvent struct {
	Agent Agent
}

func (e AgentUpdatedEvent) EventName() string {
	return "agent.updated"
}

type AgentDeletedEvent struct {
	AgentID uuid.UUID
}

func (e AgentDeletedEvent) EventName() string {
	return "agent.deleted"
}

type AgentLastSeenUpdatedEvent struct {
	AgentID uuid.UUID
	HostID  uuid.UUID
}

func (e AgentLastSeenUpdatedEvent) EventName() string {
	return "agent.last_seen.updated"
}
