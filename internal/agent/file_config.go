package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.yaml.in/yaml/v4"
)

var DefaultConfigPath = resolveDefaultAgentConfigPath()

type fileAgentConfig struct {
	AgentID    string                    `yaml:"agent_id"`
	Connection fileAgentConnectionConfig `yaml:"connection"`
	Heartbeat  fileAgentTickerConfig     `yaml:"heartbeat"`
	Metrics    fileAgentTickerConfig     `yaml:"metrics"`
	Version    int                       `yaml:"version"`
}

type fileAgentConnectionConfig struct {
	Host   string `yaml:"host"`
	Scheme string `yaml:"scheme"`
	APIKey string `yaml:"api_key"`
}

type fileAgentTickerConfig struct {
	Interval string `yaml:"interval"`
}

func resolveDefaultAgentConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return ".hosthalla/agent.yaml"
	}

	return filepath.Join(homeDir, ".hosthalla", "agent.yaml")
}

func LoadConfigFromPath(path string) (*AgentConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read agent config %q: %w", path, err)
	}

	var fileCfg fileAgentConfig
	if err := yaml.Unmarshal(raw, &fileCfg); err != nil {
		return nil, fmt.Errorf("unmarshal agent config %q: %w", path, err)
	}

	cfg, err := fileCfg.toAgentConfig()
	if err != nil {
		return nil, fmt.Errorf("parse agent config %q: %w", path, err)
	}
	return cfg, nil
}

func SaveConfigToPath(path string, cfg *AgentConfig) error {
	if cfg == nil {
		return errors.New("agent config is nil")
	}

	fileCfg := fromAgentConfig(cfg)
	raw, err := yaml.Marshal(fileCfg)
	if err != nil {
		return fmt.Errorf("marshal agent config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory for %q: %w", path, err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return fmt.Errorf("write agent config %q: %w", path, err)
	}
	return nil
}

func fromAgentConfig(cfg *AgentConfig) fileAgentConfig {
	return fileAgentConfig{
		AgentID: cfg.AgentID.String(),
		Connection: fileAgentConnectionConfig{
			Host:   cfg.Connection.Host,
			Scheme: cfg.Connection.Scheme,
			APIKey: cfg.Connection.APIKey,
		},
		Heartbeat: fileAgentTickerConfig{
			Interval: cfg.Heartbeat.Interval.String(),
		},
		Metrics: fileAgentTickerConfig{
			Interval: cfg.Metrics.Interval.String(),
		},
		Version: cfg.Version,
	}
}

func (f fileAgentConfig) toAgentConfig() (*AgentConfig, error) {
	cfg := NewAgentConfig()

	agentID, err := uuid.Parse(strings.TrimSpace(f.AgentID))
	if err != nil {
		return nil, fmt.Errorf("invalid agent_id: %w", err)
	}
	cfg.AgentID = agentID

	cfg.Connection = AgentConnectionConfig{
		Host:   strings.TrimSpace(f.Connection.Host),
		Scheme: strings.TrimSpace(f.Connection.Scheme),
		APIKey: strings.TrimSpace(f.Connection.APIKey),
	}

	if cfg.Connection.Host == "" {
		return nil, errors.New("connection.host is required")
	}
	if cfg.Connection.Scheme == "" {
		return nil, errors.New("connection.scheme is required")
	}
	if cfg.Connection.APIKey == "" {
		return nil, errors.New("connection.api_key is required")
	}

	cfg.Version = f.Version
	if cfg.Version <= 0 {
		cfg.Version = 1
	}

	cfg.Heartbeat.Interval, err = parseIntervalOrDefault(f.Heartbeat.Interval, DefaultAgentHeartbeatInterval)
	if err != nil {
		return nil, fmt.Errorf("invalid heartbeat.interval: %w", err)
	}

	cfg.Metrics.Interval, err = parseIntervalOrDefault(f.Metrics.Interval, DefaultAgentMetricsInterval)
	if err != nil {
		return nil, fmt.Errorf("invalid metrics.interval: %w", err)
	}

	return cfg, nil
}

func parseIntervalOrDefault(raw string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	if parsed <= 0 {
		return 0, errors.New("must be greater than zero")
	}
	return parsed, nil
}
