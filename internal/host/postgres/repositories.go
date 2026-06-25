package postgres

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yazmeyaa/hosthalla/internal/host"
)

type Repositories struct {
	Host                 host.HostRepository
	HostManagementMethod host.HostManagementMethodRepository
	HostNote             host.HostNoteRepository
	HostSystemInfo       host.HostSystemInfoRepository
	HostMetricSnapshot   host.HostMetricSnapshotRepository
}

func NewRepositories(pool *pgxpool.Pool) Repositories {
	return Repositories{
		Host:                 NewHostRepository(pool),
		HostManagementMethod: NewHostManagementMethodRepository(pool),
		HostNote:             NewHostNoteRepository(pool),
		HostSystemInfo:       NewHostSystemInfoRepository(pool),
		HostMetricSnapshot:   NewHostMetricSnapshotRepository(pool),
	}
}
