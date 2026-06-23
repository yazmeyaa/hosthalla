package service

import (
	"context"
	"errors"
	"os/exec"
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
	hostRepository storage.HostRepository
}

func New(hostRepository storage.HostRepository) *Service {
	return &Service{hostRepository: hostRepository}
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
