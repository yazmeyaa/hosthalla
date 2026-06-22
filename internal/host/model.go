package host

import (
	"net/netip"
	"time"

	"github.com/google/uuid"
)

type HostID uuid.UUID
type HostNoteID uuid.UUID

func NewHostID() HostID {
	id := uuid.New()
	return HostID(id)
}
func (id HostID) String() string {
	return uuid.UUID(id).String()
}

type Host struct {
	ID          HostID
	Name        string
	Description string
	Tags        []string
	IP          netip.Addr
	Port        uint16
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type SSHPasswordCredential struct {
	ID        uuid.UUID
	HostID    HostID
	User      string
	Password  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type HostNote struct {
	ID        uuid.UUID
	HostID    HostID
	Title     string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
}
