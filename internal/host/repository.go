package host

import (
	"context"
	"net/netip"

	"github.com/google/uuid"
)

type CreateHostDTO struct {
	Name        string
	Description string
	Tags        []string
	IP          netip.Addr
}

type CreateHostManagementMethodDTO struct {
	Type        HostManagementMethodType
	Username    string
	Port        uint16
	Secret      []byte
	Description string
}

type ListHostsFilter struct {
	Tags []string
}

type HostRepository interface {
	ListHosts(ctx context.Context, filter ListHostsFilter) ([]Host, error)
	ListTags(ctx context.Context) ([]Tag, error)
	GetHostByID(ctx context.Context, hostID uuid.UUID) (Host, error)
	DeleteHost(ctx context.Context, hostID uuid.UUID) error
	UpdateHost(ctx context.Context, host *Host) error
	CreateHost(ctx context.Context, data CreateHostDTO) (Host, error)
}

type HostManagementMethodRepository interface {
	ListHostManagementMethods(ctx context.Context, hostID uuid.UUID) ([]HostManagementMethod, error)
	ListHostManagementMethodsByHostIDs(ctx context.Context, hostIDs []uuid.UUID) (map[uuid.UUID][]HostManagementMethod, error)
	GetHostManagementMethodByID(ctx context.Context, methodID uuid.UUID) (HostManagementMethod, error)
	CreateHostManagementMethod(ctx context.Context, hostID uuid.UUID, data CreateHostManagementMethodDTO) (HostManagementMethod, error)
}

type HostSystemInfoRepository interface {
	GetHostSystemInfoByHostID(ctx context.Context, hostID uuid.UUID) (HostSystemInfo, error)
	ListHostSystemInfosByHostIDs(ctx context.Context, hostIDs []uuid.UUID) (map[uuid.UUID]HostSystemInfo, error)
	UpsertHostSystemInfo(ctx context.Context, data HostSystemInfo) (HostSystemInfo, error)
}

type HostMetricSnapshotRepository interface {
	ListHostMetricSnapshots(ctx context.Context, hostID uuid.UUID) ([]HostMetricSnapshot, error)
	ListLatestHostMetricSnapshotsByHostIDs(ctx context.Context, hostIDs []uuid.UUID) (map[uuid.UUID]HostMetricSnapshot, error)
	CreateHostMetricSnapshot(ctx context.Context, data HostMetricSnapshot) (HostMetricSnapshot, error)
}
