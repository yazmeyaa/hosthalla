package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yazmeyaa/hosthalla/internal/events"
)

type PingResult struct {
	HostID       uuid.UUID
	IP           string
	Reachable    bool
	Duration     time.Duration
	ErrorMessage string
	CheckedAt    time.Time
}

type Service struct {
	hostRepository                 HostRepository
	hostManagementMethodRepository HostManagementMethodRepository
	hostSystemInfoRepository       HostSystemInfoRepository
	hostMetricSnapshotRepository   HostMetricSnapshotRepository
	secretCipher                   SecretCipher
	logger                         *slog.Logger
	eventBus                       events.EventBus
}

type NewServiceParams struct {
	HostRepository                 HostRepository
	HostManagementMethodRepository HostManagementMethodRepository
	HostSystemInfoRepository       HostSystemInfoRepository
	HostMetricSnapshotRepository   HostMetricSnapshotRepository
	SecretCipher                   SecretCipher
	Logger                         *slog.Logger
	EventBus                       events.EventBus
}

func NewService(
	params NewServiceParams,
) *Service {
	eventBus := params.EventBus
	if eventBus == nil {
		eventBus = events.NewInMemoryEventBus()
	}

	return &Service{
		hostRepository:                 params.HostRepository,
		hostManagementMethodRepository: params.HostManagementMethodRepository,
		hostSystemInfoRepository:       params.HostSystemInfoRepository,
		hostMetricSnapshotRepository:   params.HostMetricSnapshotRepository,
		secretCipher:                   params.SecretCipher,
		logger:                         params.Logger,
		eventBus:                       eventBus,
	}
}

func (s *Service) ListHosts(ctx context.Context, filter ListHostsFilter) ([]Host, error) {
	filter.Tags = normalizeTags(filter.Tags)
	hosts, err := s.hostRepository.ListHosts(ctx, filter)
	if err != nil {
		s.logger.Error("failed to list hosts", slog.Any("tags", filter.Tags), slog.String("error", err.Error()))
		return nil, err
	}
	s.logger.Debug("listed hosts", slog.Int("count", len(hosts)), slog.Any("tags", filter.Tags))
	return hosts, nil
}

func (s *Service) ListTags(ctx context.Context) ([]Tag, error) {
	tags, err := s.hostRepository.ListTags(ctx)
	if err != nil {
		s.logger.Error("failed to list tags", slog.String("error", err.Error()))
		return nil, err
	}
	s.logger.Debug("listed tags", slog.Int("count", len(tags)))
	return tags, nil
}

func (s *Service) GetHostByID(ctx context.Context, hostID uuid.UUID) (Host, error) {
	result, err := s.hostRepository.GetHostByID(ctx, hostID)
	if err != nil {
		s.logger.Error("failed to get host by id", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
		return Host{}, err
	}
	s.logger.Debug("loaded host by id", slog.String("host_id", hostID.String()))
	return result, nil
}

func (s *Service) CreateHost(ctx context.Context, data CreateHostDTO) (Host, error) {
	data.Tags = normalizeTags(data.Tags)
	createdHost, err := s.hostRepository.CreateHost(ctx, data)
	if err != nil {
		s.logger.Error("failed to create host", slog.String("name", data.Name), slog.String("ip", data.IP.String()), slog.String("error", err.Error()))
		return Host{}, err
	}
	s.logger.Info("host created", slog.String("host_id", createdHost.ID.String()), slog.String("name", createdHost.Name))
	if err := s.eventBus.Publish(ctx, CreateHostEvent{Host: createdHost}); err != nil {
		s.logger.Error("failed to publish host created event", slog.String("host_id", createdHost.ID.String()), slog.String("error", err.Error()))
	}
	return createdHost, nil
}

func (s *Service) UpdateHost(ctx context.Context, target *Host) error {
	target.Tags = normalizeTags(target.Tags)
	if err := s.hostRepository.UpdateHost(ctx, target); err != nil {
		s.logger.Error("failed to update host", slog.String("host_id", target.ID.String()), slog.String("error", err.Error()))
		return err
	}
	s.logger.Info("host updated", slog.String("host_id", target.ID.String()), slog.String("name", target.Name))
	if err := s.eventBus.Publish(ctx, UpdateHostEvent{Host: *target}); err != nil {
		s.logger.Error("failed to publish host updated event", slog.String("host_id", target.ID.String()), slog.String("error", err.Error()))
	}
	return nil
}

func (s *Service) AssignMonitoringAgent(ctx context.Context, hostID uuid.UUID, agentID uuid.UUID) error {
	targetHost, err := s.GetHostByID(ctx, hostID)
	if err != nil {
		return err
	}
	targetHost.MonitoringAgentID = agentID
	if err := s.UpdateHost(ctx, &targetHost); err != nil {
		return err
	}
	if err := s.eventBus.Publish(ctx, HostMonitoringAgentAssignedEvent{HostID: hostID, AgentID: agentID}); err != nil {
		s.logger.Error("failed to publish host monitoring agent assigned event", slog.String("host_id", hostID.String()), slog.String("agent_id", agentID.String()), slog.String("error", err.Error()))
	}
	return nil
}

func (s *Service) DeleteHost(ctx context.Context, hostID uuid.UUID) error {
	if err := s.hostRepository.DeleteHost(ctx, hostID); err != nil {
		s.logger.Error("failed to delete host", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
		return err
	}
	s.logger.Warn("host deleted", slog.String("host_id", hostID.String()))
	if err := s.eventBus.Publish(ctx, DeleteHostEvent{HostID: hostID}); err != nil {
		s.logger.Error("failed to publish host deleted event", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
	}
	return nil
}

func (s *Service) ListHostManagementMethods(ctx context.Context, hostID uuid.UUID) ([]HostManagementMethod, error) {
	methods, err := s.hostManagementMethodRepository.ListHostManagementMethods(ctx, hostID)
	if err != nil {
		s.logger.Error("failed to list host management methods", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
		return nil, err
	}
	s.logger.Debug("listed host management methods", slog.String("host_id", hostID.String()), slog.Int("count", len(methods)))
	for idx := range methods {
		methods[idx].Secret = nil
	}
	return methods, nil
}

func (s *Service) ListHostManagementMethodsByHostIDs(ctx context.Context, hostIDs []uuid.UUID) (map[uuid.UUID][]HostManagementMethod, error) {
	methodsByHostID, err := s.hostManagementMethodRepository.ListHostManagementMethodsByHostIDs(ctx, hostIDs)
	if err != nil {
		s.logger.Error("failed to list host management methods by host ids", slog.String("error", err.Error()))
		return nil, err
	}
	for hostID, methods := range methodsByHostID {
		for idx := range methods {
			methods[idx].Secret = nil
		}
		methodsByHostID[hostID] = methods
	}
	return methodsByHostID, nil
}

func (s *Service) GetHostSystemInfoByHostID(ctx context.Context, hostID uuid.UUID) (HostSystemInfo, error) {
	systemInfo, err := s.hostSystemInfoRepository.GetHostSystemInfoByHostID(ctx, hostID)
	if err != nil {
		s.logger.Error("failed to get host system info by host id", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
		return HostSystemInfo{}, err
	}
	s.logger.Debug("loaded host system info", slog.String("host_id", hostID.String()))
	return systemInfo, nil
}

func (s *Service) ListHostSystemInfosByHostIDs(ctx context.Context, hostIDs []uuid.UUID) (map[uuid.UUID]HostSystemInfo, error) {
	result, err := s.hostSystemInfoRepository.ListHostSystemInfosByHostIDs(ctx, hostIDs)
	if err != nil {
		s.logger.Error("failed to list host system infos by host ids", slog.String("error", err.Error()))
		return nil, err
	}
	return result, nil
}

func (s *Service) UpsertHostSystemInfo(ctx context.Context, data HostSystemInfo) (HostSystemInfo, error) {
	systemInfo, err := s.hostSystemInfoRepository.UpsertHostSystemInfo(ctx, data)
	if err != nil {
		s.logger.Error("failed to upsert host system info", slog.String("host_id", data.HostID.String()), slog.String("error", err.Error()))
		return HostSystemInfo{}, err
	}
	s.logger.Info("host system info upserted", slog.String("host_id", data.HostID.String()))
	if err := s.eventBus.Publish(ctx, HostSystemInfoUpdatedEvent{HostID: data.HostID, Info: systemInfo}); err != nil {
		s.logger.Error("failed to publish host system info updated event", slog.String("host_id", data.HostID.String()), slog.String("error", err.Error()))
	}
	return systemInfo, nil
}

func (s *Service) ListHostMetricSnapshots(ctx context.Context, hostID uuid.UUID) ([]HostMetricSnapshot, error) {
	snapshots, err := s.hostMetricSnapshotRepository.ListHostMetricSnapshots(ctx, hostID)
	if err != nil {
		s.logger.Error("failed to list host metric snapshots", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
		return nil, err
	}
	s.logger.Debug("listed host metric snapshots", slog.String("host_id", hostID.String()), slog.Int("count", len(snapshots)))
	return snapshots, nil
}

func (s *Service) ListLatestHostMetricSnapshotsByHostIDs(ctx context.Context, hostIDs []uuid.UUID) (map[uuid.UUID]HostMetricSnapshot, error) {
	result, err := s.hostMetricSnapshotRepository.ListLatestHostMetricSnapshotsByHostIDs(ctx, hostIDs)
	if err != nil {
		s.logger.Error("failed to list latest host metric snapshots by host ids", slog.String("error", err.Error()))
		return nil, err
	}
	return result, nil
}

func (s *Service) CreateHostMetricSnapshot(ctx context.Context, data HostMetricSnapshot) (HostMetricSnapshot, error) {
	createdSnapshot, err := s.hostMetricSnapshotRepository.CreateHostMetricSnapshot(ctx, data)
	if err != nil {
		s.logger.Error("failed to create host metric snapshot", slog.String("host_id", data.HostID.String()), slog.String("error", err.Error()))
		return HostMetricSnapshot{}, err
	}
	s.logger.Info("host metric snapshot created", slog.String("host_id", data.HostID.String()), slog.String("timestamp", createdSnapshot.Timestamp.Format(time.RFC3339)))
	for _, metric := range createdSnapshot.Metrics {
		if err := s.eventBus.Publish(ctx, HostMetricReceivedEvent{HostID: data.HostID, Metric: metric}); err != nil {
			s.logger.Error("failed to publish host metric received event", slog.String("host_id", data.HostID.String()), slog.String("error", err.Error()))
		}
	}
	if err := s.eventBus.Publish(ctx, HostMetricSnapshotCreatedEvent{HostID: data.HostID, Snapshot: createdSnapshot}); err != nil {
		s.logger.Error("failed to publish host metric snapshot created event", slog.String("host_id", data.HostID.String()), slog.String("error", err.Error()))
	}
	return createdSnapshot, nil
}

type CreateSSHPasswordManagementMethodDTO struct {
	Username    string
	Password    string
	Port        uint16
	Description string
}

func (s *Service) CreateSSHPasswordManagementMethod(ctx context.Context, hostID uuid.UUID, data CreateSSHPasswordManagementMethodDTO) (HostManagementMethod, error) {
	username := strings.TrimSpace(data.Username)
	password := strings.TrimSpace(data.Password)
	if username == "" {
		s.logger.Warn("failed to create ssh password method: username is required", slog.String("host_id", hostID.String()))
		return HostManagementMethod{}, errors.New("username is required")
	}
	if password == "" {
		s.logger.Warn("failed to create ssh password method: password is required", slog.String("host_id", hostID.String()))
		return HostManagementMethod{}, errors.New("password is required")
	}

	encryptedSecret, err := s.secretCipher.Encrypt([]byte(password))
	if err != nil {
		s.logger.Error("failed to encrypt ssh password secret", slog.String("host_id", hostID.String()), slog.String("username", username), slog.String("error", err.Error()))
		return HostManagementMethod{}, fmt.Errorf("failed to encrypt secret: %w", err)
	}
	method, err := s.hostManagementMethodRepository.CreateHostManagementMethod(ctx, hostID, CreateHostManagementMethodDTO{
		Type:        HostManagementMethodTypeSSHPassword,
		Username:    username,
		Port:        normalizePort(data.Port),
		Secret:      encryptedSecret,
		Description: strings.TrimSpace(data.Description),
	})
	if err != nil {
		s.logger.Error("failed to create ssh password method", slog.String("host_id", hostID.String()), slog.String("username", username), slog.String("error", err.Error()))
		return HostManagementMethod{}, err
	}
	s.logger.Info("created ssh password method", slog.String("host_id", hostID.String()), slog.String("method_id", method.ID.String()), slog.String("username", username))
	s.publishHostManagementMethodCreated(ctx, hostID, method)
	return method, nil
}

type CreateSSHKeyManagementMethodDTO struct {
	Username    string
	PublicKey   string
	PrivateKey  string
	Port        uint16
	Description string
}

func (s *Service) CreateSSHKeyManagementMethod(ctx context.Context, hostID uuid.UUID, data CreateSSHKeyManagementMethodDTO) (HostManagementMethod, error) {
	username := strings.TrimSpace(data.Username)
	publicKey := strings.TrimSpace(data.PublicKey)
	privateKey := strings.TrimSpace(data.PrivateKey)
	if username == "" {
		s.logger.Warn("failed to create ssh key method: username is required", slog.String("host_id", hostID.String()))
		return HostManagementMethod{}, errors.New("username is required")
	}
	if publicKey == "" {
		s.logger.Warn("failed to create ssh key method: public key is required", slog.String("host_id", hostID.String()))
		return HostManagementMethod{}, errors.New("public key is required")
	}
	if privateKey == "" {
		s.logger.Warn("failed to create ssh key method: private key is required", slog.String("host_id", hostID.String()))
		return HostManagementMethod{}, errors.New("private key is required")
	}

	secretRaw, err := json.Marshal(struct {
		PublicKey  string `json:"publicKey"`
		PrivateKey string `json:"privateKey"`
	}{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	})
	if err != nil {
		s.logger.Error("failed to prepare ssh key secret", slog.String("host_id", hostID.String()), slog.String("username", username), slog.String("error", err.Error()))
		return HostManagementMethod{}, fmt.Errorf("failed to prepare ssh key secret: %w", err)
	}

	encryptedSecret, err := s.secretCipher.Encrypt(secretRaw)
	if err != nil {
		s.logger.Error("failed to encrypt ssh key secret", slog.String("host_id", hostID.String()), slog.String("username", username), slog.String("error", err.Error()))
		return HostManagementMethod{}, fmt.Errorf("failed to encrypt secret: %w", err)
	}

	method, err := s.hostManagementMethodRepository.CreateHostManagementMethod(ctx, hostID, CreateHostManagementMethodDTO{
		Type:        HostManagementMethodTypeSSHKey,
		Username:    username,
		Port:        normalizePort(data.Port),
		Secret:      encryptedSecret,
		Description: strings.TrimSpace(data.Description),
	})
	if err != nil {
		s.logger.Error("failed to create ssh key method", slog.String("host_id", hostID.String()), slog.String("username", username), slog.String("error", err.Error()))
		return HostManagementMethod{}, err
	}
	s.logger.Info("created ssh key method", slog.String("host_id", hostID.String()), slog.String("method_id", method.ID.String()), slog.String("username", username))
	s.publishHostManagementMethodCreated(ctx, hostID, method)
	return method, nil
}

func (s *Service) PingHost(ctx context.Context, hostID uuid.UUID) (PingResult, error) {
	targetHost, err := s.hostRepository.GetHostByID(ctx, hostID)
	if err != nil {
		s.logger.Error("failed to load host before ping", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
		return PingResult{}, err
	}

	result, err := s.pingHost(ctx, targetHost)
	if err != nil {
		s.logger.Error("failed to ping host", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
		return PingResult{}, err
	}
	if result.Reachable {
		s.logger.Info("host is reachable", slog.String("host_id", hostID.String()), slog.Int64("duration_ms", result.Duration.Milliseconds()))
	} else {
		s.logger.Warn("host is unreachable", slog.String("host_id", hostID.String()), slog.String("reason", result.ErrorMessage))
	}
	if err := s.eventBus.Publish(ctx, HostPingCompletedEvent{Result: result}); err != nil {
		s.logger.Error("failed to publish host ping completed event", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
	}
	return result, nil
}

func (s *Service) PingAllHosts(ctx context.Context) ([]PingResult, error) {
	hosts, err := s.hostRepository.ListHosts(ctx, ListHostsFilter{})
	if err != nil {
		s.logger.Error("failed to list hosts before ping all", slog.String("error", err.Error()))
		return nil, err
	}

	results := make([]PingResult, 0, len(hosts))
	for _, currentHost := range hosts {
		result, err := s.pingHost(ctx, currentHost)
		if err != nil {
			s.logger.Error("failed to ping host in batch", slog.String("host_id", currentHost.ID.String()), slog.String("error", err.Error()))
			return nil, err
		}
		results = append(results, result)
	}
	s.logger.Info("completed ping all hosts", slog.Int("total", len(results)))
	if err := s.eventBus.Publish(ctx, HostsPingCompletedEvent{Results: results}); err != nil {
		s.logger.Error("failed to publish hosts ping completed event", slog.String("error", err.Error()))
	}

	return results, nil
}

func (s *Service) publishHostManagementMethodCreated(ctx context.Context, hostID uuid.UUID, method HostManagementMethod) {
	method.Secret = nil
	if err := s.eventBus.Publish(ctx, HostManagementMethodCreatedEvent{HostID: hostID, Method: method}); err != nil {
		s.logger.Error("failed to publish host management method created event", slog.String("host_id", hostID.String()), slog.String("method_id", method.ID.String()), slog.String("error", err.Error()))
	}
}

func (s *Service) pingHost(ctx context.Context, targetHost Host) (PingResult, error) {
	startedAt := time.Now()
	cmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", "1", targetHost.IP.String())
	output, err := cmd.CombinedOutput()
	result := PingResult{
		HostID:    targetHost.ID,
		IP:        targetHost.IP.String(),
		Duration:  time.Since(startedAt),
		CheckedAt: time.Now(),
	}
	if err == nil {
		result.Reachable = true
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ErrorMessage = pingFailureMessage(string(output))
		return result, nil
	}

	return PingResult{}, err
}

func pingFailureMessage(rawOutput string) string {
	output := strings.ToLower(rawOutput)
	switch {
	case strings.Contains(output, "100% packet loss"):
		return "No ICMP response (100% packet loss)."
	case strings.Contains(output, "name or service not known"):
		return "Cannot resolve host address."
	case strings.Contains(output, "network is unreachable"):
		return "Network is unreachable."
	default:
		return "Host is unreachable via ICMP."
	}
}

func ParsePort(rawPort string) (uint16, error) {
	trimmedPort := strings.TrimSpace(rawPort)
	if trimmedPort == "" {
		return 22, nil
	}
	portInt, err := strconv.Atoi(trimmedPort)
	if err != nil {
		return 0, err
	}
	if portInt < 1 || portInt > 65535 {
		return 0, errors.New("port must be between 1 and 65535")
	}
	return uint16(portInt), nil
}

func normalizePort(port uint16) uint16 {
	if port == 0 {
		return 22
	}
	return port
}

func normalizeTags(tags []string) []string {
	result := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}
