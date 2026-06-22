package storage

import (
	"context"
	"net/netip"

	"github.com/yazmeyaa/hosthalla/internal/host"
)

type CreateHostDTO struct {
	Name        string
	Description string
	Tags        []string
	IP          netip.Addr
	Port        uint16
}

type CreateHostNoteDTO struct {
	Title string
	Body  string
}

type HostRepository interface {
	ListHosts(ctx context.Context) ([]host.Host, error)
	GetHostByID(ctx context.Context, hostID host.HostID) (host.Host, error)
	DeleteHost(ctx context.Context, hostID host.HostID) error
	UpdateHost(ctx context.Context, host *host.Host) error
	CreateHost(ctx context.Context, data CreateHostDTO) (host.Host, error)
}

type HostNoteRepository interface {
	ListHostNotes(ctx context.Context, hostID host.HostID) ([]host.HostNote, error)
	GetHostNodeByID(ctx context.Context, hostNoteID host.HostNoteID) (host.HostNote, error)
	CreateHostNote(ctx context.Context, hostID host.HostID, data CreateHostNoteDTO) (host.HostNote, error)
	DeleteHostNote(ctx context.Context, hostNoteID host.HostNoteID) error
	UpdateHostNote(ctx context.Context, hostNote *host.HostNote) error
}
