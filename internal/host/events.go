package host

import "github.com/google/uuid"

type HostMetricReceivedEvent struct {
	HostID uuid.UUID
	Metric HostMetric
}

func (e HostMetricReceivedEvent) EventName() string {
	return "host.metric.received"
}

type HostMonitoringAgentAssignedEvent struct {
	HostID  uuid.UUID
	AgentID uuid.UUID
}

func (e HostMonitoringAgentAssignedEvent) EventName() string {
	return "host.monitoring_agent.assigned"
}

type HostManagementMethodCreatedEvent struct {
	HostID uuid.UUID
	Method HostManagementMethod
}

func (e HostManagementMethodCreatedEvent) EventName() string {
	return "host.management_method.created"
}

type HostPingCompletedEvent struct {
	Result PingResult
}

func (e HostPingCompletedEvent) EventName() string {
	return "host.ping.completed"
}

type HostsPingCompletedEvent struct {
	Results []PingResult
}

func (e HostsPingCompletedEvent) EventName() string {
	return "host.ping_all.completed"
}

type HostSystemInfoUpdatedEvent struct {
	HostID uuid.UUID
	Info   HostSystemInfo
}

func (e HostSystemInfoUpdatedEvent) EventName() string {
	return "host.system.info.updated"
}

type HostMetricSnapshotCreatedEvent struct {
	HostID   uuid.UUID
	Snapshot HostMetricSnapshot
}

func (e HostMetricSnapshotCreatedEvent) EventName() string {
	return "host.metric.snapshot.created"
}

type CreateHostEvent struct {
	Host Host
}

func (e CreateHostEvent) EventName() string {
	return "host.created"
}

type UpdateHostEvent struct {
	Host Host
}

func (e UpdateHostEvent) EventName() string {
	return "host.updated"
}

type DeleteHostEvent struct {
	HostID uuid.UUID
}

func (e DeleteHostEvent) EventName() string {
	return "host.deleted"
}
