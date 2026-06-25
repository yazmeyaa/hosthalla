package agent

import (
	"time"

	"github.com/google/uuid"
	"github.com/yazmeyaa/hosthalla/internal/host"
)

type AgentID uuid.UUID

type Agent struct {
	ID     AgentID
	HostID host.HostID

	Version string

	CreatedAt  time.Time
	LastSeenAt time.Time
}
