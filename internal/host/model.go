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
	ID          uuid.UUID
	Name        string
	Description string
	Tags        []string
	IP          netip.Addr

	MonitoringAgentID uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time
}

type Tag struct {
	ID        uuid.UUID
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SSHPasswordCredential struct {
	ID        uuid.UUID
	HostID    HostID
	User      string
	Port      uint16
	Password  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type HostManagementMethodType string

const (
	HostManagementMethodTypeSSHPassword HostManagementMethodType = "ssh_password"
	HostManagementMethodTypeSSHKey      HostManagementMethodType = "ssh_key"
)

type HostManagementMethod struct {
	ID          uuid.UUID
	HostID      HostID
	Type        HostManagementMethodType
	Username    string
	Port        uint16
	Secret      []byte
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type HostNote struct {
	ID        uuid.UUID
	HostID    uuid.UUID
	Title     string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type HostSystemInfo struct {
	HostID uuid.UUID

	Hostname         string
	OS               OSSystemInfo
	TotalMemoryBytes uint64
	CPU              CPUSystemInfo
	GPUs             []GPUSystemInfo
	TotalDiskBytes   uint64
}

type GPUSystemInfo struct {
	Name string
}

type OSSystemInfo struct {
	Name    string
	Version string
	Kernel  string
}

type CPUSystemInfo struct {
	Name         string
	Architecture string
	Cores        uint
	Frequency    float64
	Threads      uint
}

type HostMetricSnapshot struct {
	HostID    HostID
	Timestamp time.Time
	Metrics   []HostMetric
}

type HostMetric struct {
	CPUUsagePercentage float64
	MemoryUsageBytes   uint64
	DiskUsageBytes     uint64
	NetworkRxBytes     uint64
	NetworkTxBytes     uint64
}
