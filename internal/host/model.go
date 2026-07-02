package host

import (
	"net/netip"
	"time"

	"github.com/google/uuid"
)

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
	HostID    uuid.UUID
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
	HostID      uuid.UUID
	Type        HostManagementMethodType
	Username    string
	Port        uint16
	Secret      []byte
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type HostManagementMethodSecret struct {
	Password   string
	PublicKey  string
	PrivateKey string
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
	HostID    uuid.UUID
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
