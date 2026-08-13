package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.yaml.in/yaml/v4"
)

var (
	DefaultConfigDir  = resolveDefaultAgentConfigDir()
	DefaultConfigPath = filepath.Join(DefaultConfigDir, "agent.yaml")
)

// LoadedConfig is an agent configuration loaded from a configuration file.
type LoadedConfig struct {
	Name   string
	Path   string
	Config *AgentConfig
}

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

func resolveDefaultAgentConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return ".hosthalla/agent.d"
	}

	return filepath.Join(homeDir, ".hosthalla", "agent.d")
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

// LoadConfigsFromDir loads all regular .yaml and .yml files directly in dir.
func LoadConfigsFromDir(dir string) ([]LoadedConfig, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve agent config directory %q: %w", dir, err)
	}
	absDir = filepath.Clean(absDir)

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, fmt.Errorf("read agent config directory %q: %w", absDir, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	configs := make([]LoadedConfig, 0, len(entries))
	var errs []error
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !isAgentConfigFile(entry.Name()) {
			continue
		}

		path := filepath.Join(absDir, entry.Name())
		cfg, err := LoadConfigFromPath(path)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		configs = append(configs, LoadedConfig{
			Name:   strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
			Path:   path,
			Config: cfg,
		})
	}
	if len(configs) == 0 {
		if len(errs) > 0 {
			return nil, errors.Join(errs...)
		}
		return nil, fmt.Errorf("agent config directory %q contains no .yaml or .yml files", absDir)
	}

	seen := make(map[string]string, len(configs))
	for _, loaded := range configs {
		key := normalizedEndpoint(loaded.Config) + "\x00" + loaded.Config.AgentID.String()
		if firstPath, ok := seen[key]; ok {
			errs = append(errs, fmt.Errorf("duplicate agent config for %s and agent %s: %s and %s", normalizedEndpoint(loaded.Config), loaded.Config.AgentID, firstPath, loaded.Path))
			continue
		}
		seen[key] = loaded.Path
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return configs, nil
}

func isAgentConfigFile(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".yaml" || ext == ".yml"
}

func normalizedEndpoint(cfg *AgentConfig) string {
	return strings.ToLower(strings.TrimSpace(cfg.Connection.Scheme)) + "://" + strings.ToLower(strings.TrimSpace(cfg.Connection.Host))
}

func SaveConfigToPath(path string, cfg *AgentConfig) error {
	return saveConfigToPath(path, cfg, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
}

// SaveNewConfigToPath creates a config without replacing an existing file.
func SaveNewConfigToPath(path string, cfg *AgentConfig) error {
	return saveConfigToPath(path, cfg, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
}

func saveConfigToPath(path string, cfg *AgentConfig, flags int) error {
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
	file, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return fmt.Errorf("write agent config %q: %w", path, err)
	}
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		return fmt.Errorf("write agent config %q: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close agent config %q: %w", path, err)
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
