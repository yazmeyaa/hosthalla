package agent

import (
	"time"

	"github.com/google/uuid"
)

type Agent struct {
	ID     uuid.UUID
	HostID uuid.UUID

	Version string

	CreatedAt  time.Time
	LastSeenAt time.Time
}
