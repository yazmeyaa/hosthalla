package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/yazmeyaa/hosthalla/internal/host"
	"github.com/yazmeyaa/hosthalla/internal/host/storage"
)

type PingResult struct {
	HostID       host.HostID
	IP           string
	Reachable    bool
	Duration     time.Duration
	ErrorMessage string
	CheckedAt    time.Time
}

type Service struct {
	hostRepository                 storage.HostRepository
	hostManagementMethodRepository storage.HostManagementMethodRepository
}

func New(hostRepository storage.HostRepository, hostManagementMethodRepository storage.HostManagementMethodRepository) *Service {
	return &Service{
		hostRepository:                 hostRepository,
		hostManagementMethodRepository: hostManagementMethodRepository,
	}
}

func (s *Service) ListHosts(ctx context.Context) ([]host.Host, error) {
	return s.hostRepository.ListHosts(ctx)
}

func (s *Service) GetHostByID(ctx context.Context, hostID host.HostID) (host.Host, error) {
	return s.hostRepository.GetHostByID(ctx, hostID)
}

func (s *Service) CreateHost(ctx context.Context, data storage.CreateHostDTO) (host.Host, error) {
	return s.hostRepository.CreateHost(ctx, data)
}

func (s *Service) UpdateHost(ctx context.Context, target *host.Host) error {
	return s.hostRepository.UpdateHost(ctx, target)
}

func (s *Service) DeleteHost(ctx context.Context, hostID host.HostID) error {
	return s.hostRepository.DeleteHost(ctx, hostID)
}

func (s *Service) ListHostManagementMethods(ctx context.Context, hostID host.HostID) ([]host.HostManagementMethod, error) {
	return s.hostManagementMethodRepository.ListHostManagementMethods(ctx, hostID)
}

type CreateSSHPasswordManagementMethodDTO struct {
	Username    string
	Password    string
	Port        uint16
	Description string
}

func (s *Service) CreateSSHPasswordManagementMethod(ctx context.Context, hostID host.HostID, data CreateSSHPasswordManagementMethodDTO) (host.HostManagementMethod, error) {
	username := strings.TrimSpace(data.Username)
	password := strings.TrimSpace(data.Password)
	if username == "" {
		return host.HostManagementMethod{}, errors.New("username is required")
	}
	if password == "" {
		return host.HostManagementMethod{}, errors.New("password is required")
	}

	return s.hostManagementMethodRepository.CreateHostManagementMethod(ctx, hostID, storage.CreateHostManagementMethodDTO{
		Type:        host.HostManagementMethodTypeSSHPassword,
		Username:    username,
		Port:        normalizePort(data.Port),
		Secret:      []byte(password),
		Description: strings.TrimSpace(data.Description),
	})
}

type CreateSSHKeyManagementMethodDTO struct {
	Username    string
	PublicKey   string
	PrivateKey  string
	Port        uint16
	Description string
}

func (s *Service) CreateSSHKeyManagementMethod(ctx context.Context, hostID host.HostID, data CreateSSHKeyManagementMethodDTO) (host.HostManagementMethod, error) {
	username := strings.TrimSpace(data.Username)
	publicKey := strings.TrimSpace(data.PublicKey)
	privateKey := strings.TrimSpace(data.PrivateKey)
	if username == "" {
		return host.HostManagementMethod{}, errors.New("username is required")
	}
	if publicKey == "" {
		return host.HostManagementMethod{}, errors.New("public key is required")
	}
	if privateKey == "" {
		return host.HostManagementMethod{}, errors.New("private key is required")
	}

	secretRaw, err := json.Marshal(struct {
		PublicKey  string `json:"publicKey"`
		PrivateKey string `json:"privateKey"`
	}{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	})
	if err != nil {
		return host.HostManagementMethod{}, fmt.Errorf("failed to prepare ssh key secret: %w", err)
	}

	return s.hostManagementMethodRepository.CreateHostManagementMethod(ctx, hostID, storage.CreateHostManagementMethodDTO{
		Type:        host.HostManagementMethodTypeSSHKey,
		Username:    username,
		Port:        normalizePort(data.Port),
		Secret:      secretRaw,
		Description: strings.TrimSpace(data.Description),
	})
}

func (s *Service) PingHost(ctx context.Context, hostID host.HostID) (PingResult, error) {
	targetHost, err := s.hostRepository.GetHostByID(ctx, hostID)
	if err != nil {
		return PingResult{}, err
	}

	return s.pingHost(ctx, targetHost)
}

func (s *Service) PingAllHosts(ctx context.Context) ([]PingResult, error) {
	hosts, err := s.hostRepository.ListHosts(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]PingResult, 0, len(hosts))
	for _, currentHost := range hosts {
		result, err := s.pingHost(ctx, currentHost)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	return results, nil
}

func (s *Service) pingHost(ctx context.Context, targetHost host.Host) (PingResult, error) {
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
