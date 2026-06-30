package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	gopsutilhost "github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/yazmeyaa/hosthalla/internal/agent"
	host_model "github.com/yazmeyaa/hosthalla/internal/host"
	app_logger "github.com/yazmeyaa/hosthalla/internal/logger"
	"github.com/yazmeyaa/hosthalla/internal/version"
)

func runAgentCommand(ctx context.Context, stdout io.Writer, stderr io.Writer, args []string) error {
	if len(args) == 0 {
		printAgentUsage(stderr)
		return fmt.Errorf("agent command is required")
	}

	switch args[0] {
	case "register":
		return processAgentRegisterCommand(ctx, stdout, stderr, args[1:])
	case "run":
		return processAgentRunCommand(ctx, stdout, stderr, args[1:])
	default:
		fmt.Fprintf(stderr, "Unknown agent command %q\n", args[0])
		printAgentUsage(stderr)
		return fmt.Errorf("unknown agent command %q", args[0])
	}
}

func processAgentRegisterCommand(ctx context.Context, stdout io.Writer, stderr io.Writer, args []string) error {
	flags := flag.NewFlagSet("hosthalla agent register", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	hostValue := flags.String("host", "", "hosthalla API host (e.g. some-server.org or https://some-server.org)")
	schemeValue := flags.String("scheme", "", "connection scheme (http or https)")
	tokenValue := flags.String("token", "", "API token")
	hostIDValue := flags.String("host-id", "", "host id (UUID) to register agent for")
	configPath := flags.String("config", agent.DefaultConfigPath, "path to agent config file")

	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(stderr, "Failed to parse flags: %s\n", err)
		printAgentRegisterUsage(stderr)
		return err
	}
	if flags.NArg() != 0 {
		printAgentRegisterUsage(stderr)
		return fmt.Errorf("agent register does not accept positional arguments")
	}

	hostID, err := uuid.Parse(strings.TrimSpace(*hostIDValue))
	if err != nil {
		return fmt.Errorf("invalid --host-id value: %w", err)
	}

	scheme, host, err := normalizeConnectionHost(*hostValue, *schemeValue)
	if err != nil {
		return fmt.Errorf("invalid --host value: %w", err)
	}

	registerResponse, err := registerAgent(ctx, scheme, host, strings.TrimSpace(*tokenValue), hostID)
	if err != nil {
		return fmt.Errorf("register agent: %w", err)
	}

	systemInfo, err := collectHostSystemInfo(ctx, hostID)
	if err != nil {
		return fmt.Errorf("collect host system info: %w", err)
	}
	if err := sendHostSystemInfo(ctx, scheme, host, strings.TrimSpace(*tokenValue), systemInfo); err != nil {
		return fmt.Errorf("send host system info: %w", err)
	}

	cfg := agent.NewAgentConfig()
	cfg.AgentID = registerResponse.AgentID
	cfg.Connection = agent.AgentConnectionConfig{
		Host:   host,
		Scheme: scheme,
		APIKey: strings.TrimSpace(*tokenValue),
	}
	cfg.Version = 1

	if err := agent.SaveConfigToPath(*configPath, cfg); err != nil {
		return fmt.Errorf("write agent config: %w", err)
	}

	fmt.Fprintf(stdout, "Agent registered. Config saved at %q\n", *configPath)
	return nil
}

func processAgentRunCommand(ctx context.Context, stdout io.Writer, stderr io.Writer, args []string) error {
	flags := flag.NewFlagSet("hosthalla agent run", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	configPath := flags.String("config", agent.DefaultConfigPath, "path to agent config file")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(stderr, "Failed to parse flags: %s\n", err)
		printAgentRunUsage(stderr)
		return err
	}
	if flags.NArg() != 0 {
		printAgentRunUsage(stderr)
		return fmt.Errorf("agent run does not accept positional arguments")
	}

	cfg, err := agent.LoadConfigFromPath(*configPath)
	if err != nil {
		return fmt.Errorf("load agent config: %w", err)
	}

	logger := app_logger.NewLogger(app_logger.LoggerParams{
		Output: stdout,
	})
	logger.Info("starting agent worker", "version", version.VersionString(), "config_path", *configPath)

	argusService := agent.NewArgusService(cfg, logger)
	worker := agent.NewWorker(cfg, argusService, logger)
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	worker.Run(ctx)
	return nil
}

type agentRegisterResponse struct {
	AgentID uuid.UUID `json:"agentID"`
	HostID  uuid.UUID `json:"hostID"`
	Version string    `json:"version"`
}

func registerAgent(ctx context.Context, scheme string, host string, token string, hostID uuid.UUID) (*agentRegisterResponse, error) {
	if token == "" {
		return nil, fmt.Errorf("token is required")
	}

	registerURL := url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   fmt.Sprintf("/api/v1/hosts/%s/register-agent", hostID),
	}

	requestBody, err := json.Marshal(map[string]string{
		"version": version.VersionString(),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal register payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registerURL.String(), bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("create register request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "hosthalla-agent")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send register request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		if len(body) == 0 {
			return nil, fmt.Errorf("unexpected status code: %s", resp.Status)
		}
		return nil, fmt.Errorf("unexpected status code: %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	var response agentRegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode register response: %w", err)
	}
	return &response, nil
}

func collectHostSystemInfo(ctx context.Context, hostID uuid.UUID) (host_model.HostSystemInfo, error) {
	result := host_model.HostSystemInfo{
		HostID: hostID,
	}

	hostname, err := os.Hostname()
	if err != nil {
		return host_model.HostSystemInfo{}, fmt.Errorf("resolve hostname: %w", err)
	}
	result.Hostname = hostname

	hostInfo, err := gopsutilhost.InfoWithContext(ctx)
	if err != nil {
		return host_model.HostSystemInfo{}, fmt.Errorf("collect os info: %w", err)
	}
	result.OS = resolveOSSystemInfo(ctx, hostInfo)

	vm, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return host_model.HostSystemInfo{}, fmt.Errorf("collect memory info: %w", err)
	}
	result.TotalMemoryBytes = vm.Total

	cpuInfo, err := cpu.InfoWithContext(ctx)
	if err != nil {
		return host_model.HostSystemInfo{}, fmt.Errorf("collect cpu info: %w", err)
	}
	cpuName := ""
	cpuFrequency := float64(0)
	if len(cpuInfo) > 0 {
		cpuName = strings.TrimSpace(cpuInfo[0].ModelName)
		cpuFrequency = cpuInfo[0].Mhz
	}
	physicalCores, err := cpu.CountsWithContext(ctx, false)
	if err != nil {
		return host_model.HostSystemInfo{}, fmt.Errorf("collect cpu core count: %w", err)
	}
	logicalThreads, err := cpu.CountsWithContext(ctx, true)
	if err != nil {
		return host_model.HostSystemInfo{}, fmt.Errorf("collect cpu thread count: %w", err)
	}
	result.CPU = host_model.CPUSystemInfo{
		Name:         cpuName,
		Architecture: strings.TrimSpace(hostInfo.KernelArch),
		Cores:        uint(physicalCores),
		Frequency:    cpuFrequency,
		Threads:      uint(logicalThreads),
	}

	result.GPUs = make([]host_model.GPUSystemInfo, 0)

	diskTotal, err := totalDiskBytes(ctx)
	if err != nil {
		return host_model.HostSystemInfo{}, err
	}
	result.TotalDiskBytes = diskTotal

	return result, nil
}

func resolveOSSystemInfo(ctx context.Context, hostInfo *gopsutilhost.InfoStat) host_model.OSSystemInfo {
	name := strings.TrimSpace(hostInfo.Platform)
	version := strings.TrimSpace(hostInfo.PlatformVersion)
	kernel := strings.TrimSpace(hostInfo.KernelVersion)

	if platformName, platformFamily, platformVersion, err := gopsutilhost.PlatformInformationWithContext(ctx); err == nil {
		if name == "" {
			name = strings.TrimSpace(platformName)
			if name == "" {
				name = strings.TrimSpace(platformFamily)
			}
		}
		if version == "" {
			version = strings.TrimSpace(platformVersion)
		}
	}

	if name == "" {
		name = strings.TrimSpace(hostInfo.OS)
	}
	if version == "" {
		version = strings.TrimSpace(hostInfo.OS)
	}

	return host_model.OSSystemInfo{
		Name:    name,
		Version: version,
		Kernel:  kernel,
	}
}

func totalDiskBytes(ctx context.Context) (uint64, error) {
	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return 0, fmt.Errorf("collect disk partitions: %w", err)
	}

	seenDevices := make(map[string]struct{}, len(partitions))
	var totalBytes uint64
	for _, partition := range partitions {
		device := strings.TrimSpace(partition.Device)
		if device == "" {
			device = strings.TrimSpace(partition.Mountpoint)
		}
		if _, ok := seenDevices[device]; ok {
			continue
		}
		usage, err := disk.UsageWithContext(ctx, partition.Mountpoint)
		if err != nil {
			continue
		}
		totalBytes += usage.Total
		seenDevices[device] = struct{}{}
	}

	if totalBytes == 0 {
		rootUsage, err := disk.UsageWithContext(ctx, "/")
		if err != nil {
			return 0, fmt.Errorf("collect disk usage: %w", err)
		}
		totalBytes = rootUsage.Total
	}

	return totalBytes, nil
}

func sendHostSystemInfo(ctx context.Context, scheme string, host string, token string, payload host_model.HostSystemInfo) error {
	if token == "" {
		return fmt.Errorf("token is required")
	}

	systemInfoURL := url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   fmt.Sprintf("/api/v1/hosts/%s/system-info", payload.HostID),
	}

	requestBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal host system info payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, systemInfoURL.String(), bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("create host system info request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "hosthalla-agent")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send host system info request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if len(body) == 0 {
			return fmt.Errorf("unexpected status code: %s", resp.Status)
		}
		return fmt.Errorf("unexpected status code: %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	return nil
}

func normalizeConnectionHost(raw string, schemeOverride string) (string, string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", "", fmt.Errorf("host is required")
	}

	if schemeOverride != "" {
		normalizedScheme := strings.ToLower(strings.TrimSpace(schemeOverride))
		if normalizedScheme != "http" && normalizedScheme != "https" {
			return "", "", fmt.Errorf("unsupported scheme %q: expected http or https", schemeOverride)
		}
		return normalizedScheme, value, nil
	}

	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return "", "", err
		}
		if strings.TrimSpace(parsed.Scheme) == "" || strings.TrimSpace(parsed.Host) == "" {
			return "", "", fmt.Errorf("must contain scheme and host")
		}
		return strings.ToLower(strings.TrimSpace(parsed.Scheme)), strings.TrimSpace(parsed.Host), nil
	}

	if isLocalhostHost(value) {
		return "http", value, nil
	}

	return "https", value, nil
}

func isLocalhostHost(rawHost string) bool {
	host := strings.TrimSpace(rawHost)
	if host == "" {
		return false
	}

	if strings.Contains(host, ":") {
		parsedHost, _, err := net.SplitHostPort(host)
		if err == nil {
			host = parsedHost
		}
	}

	host = strings.Trim(host, "[]")
	lowerHost := strings.ToLower(host)
	if lowerHost == "localhost" || lowerHost == "::1" {
		return true
	}
	return strings.HasPrefix(lowerHost, "127.")
}

func printAgentUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  hosthalla agent register --host <server> --host-id <uuid> --token <token> [--scheme <http|https>] [--config <file>]")
	fmt.Fprintln(w, "  hosthalla agent run [--config <file>]")
}

func printAgentRegisterUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: hosthalla agent register --host <server> --host-id <uuid> --token <token> [--scheme <http|https>] [--config <file>]")
}

func printAgentRunUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: hosthalla agent run [--config <file>]")
}
