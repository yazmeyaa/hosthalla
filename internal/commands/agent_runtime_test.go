package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yazmeyaa/hosthalla/internal/agent"
	app_logger "github.com/yazmeyaa/hosthalla/internal/logger"
)

func TestLoadAgentConfigsSupportsSingleConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.yaml")
	cfg := agent.NewAgentConfig()
	cfg.AgentID = uuid.New()
	cfg.Connection = agent.AgentConnectionConfig{Scheme: "https", Host: "example.com", APIKey: "secret"}
	if err := agent.SaveConfigToPath(path, cfg); err != nil {
		t.Fatal(err)
	}

	configs, err := loadAgentConfigs(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 || configs[0].Name != "main" || !filepath.IsAbs(configs[0].Path) {
		t.Fatalf("configs = %#v", configs)
	}
}

func TestProcessAgentRunRejectsConfigAndConfigDir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := processAgentRunCommand(context.Background(), &stdout, &stderr, []string{
		"--config", "agent.yaml",
		"--config-dir", "agent.d",
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("error = %v", err)
	}
}

func TestProcessAgentRunRejectsEmptyConfigDir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := processAgentRunCommand(context.Background(), &stdout, &stderr, []string{"--config-dir", ""})
	if err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Fatalf("error = %v", err)
	}
}

func TestProcessAgentRunUsesConfigDirectoryByDefault(t *testing.T) {
	original := agent.DefaultConfigDir
	agent.DefaultConfigDir = t.TempDir()
	t.Cleanup(func() { agent.DefaultConfigDir = original })

	var stdout, stderr bytes.Buffer
	err := processAgentRunCommand(context.Background(), &stdout, &stderr, nil)
	if err == nil || !strings.Contains(err.Error(), "contains no .yaml or .yml files") {
		t.Fatalf("error = %v", err)
	}
}

func TestProcessAgentRunExplicitConfigOverridesDefaultDirectory(t *testing.T) {
	original := agent.DefaultConfigDir
	agent.DefaultConfigDir = t.TempDir()
	t.Cleanup(func() { agent.DefaultConfigDir = original })
	missing := filepath.Join(t.TempDir(), "missing.yaml")

	var stdout, stderr bytes.Buffer
	err := processAgentRunCommand(context.Background(), &stdout, &stderr, []string{"--config", missing})
	if err == nil || !strings.Contains(err.Error(), missing) {
		t.Fatalf("error = %v", err)
	}
}

func TestProcessAgentRegisterDoesNotContactServerWhenConfigExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.yaml")
	if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var stdout, stderr bytes.Buffer
	err := processAgentRegisterCommand(ctx, &stdout, &stderr, []string{
		"--config", path,
		"--host", "192.0.2.1",
		"--host-id", uuid.NewString(),
		"--token", "secret",
	})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v", err)
	}
}

func TestMinimumMetricsInterval(t *testing.T) {
	first, second := agent.NewAgentConfig(), agent.NewAgentConfig()
	first.Metrics.Interval = 10 * time.Second
	second.Metrics.Interval = 3 * time.Second
	got := minimumMetricsInterval([]agent.LoadedConfig{{Config: first}, {Config: second}})
	if got != 3*time.Second {
		t.Fatalf("minimumMetricsInterval() = %s", got)
	}
}

func TestAgentInstanceLoggerIncludesIdentityWithoutAPIKey(t *testing.T) {
	var output bytes.Buffer
	cfg := agent.NewAgentConfig()
	cfg.AgentID = uuid.New()
	cfg.Connection = agent.AgentConnectionConfig{Scheme: "https", Host: "demo.example.com", APIKey: "do-not-log"}
	loaded := agent.LoadedConfig{Name: "demo", Path: "/etc/hosthalla/agent.d/demo.yaml", Config: cfg}

	agentInstanceLogger(app_logger.NewLogger(app_logger.LoggerParams{Output: &output}), loaded).Info("test")
	logLine := output.String()
	for _, want := range []string{"config=demo", "config_path=/etc/hosthalla/agent.d/demo.yaml", "server=https://demo.example.com", "agent_id=" + cfg.AgentID.String()} {
		if !strings.Contains(logLine, want) {
			t.Fatalf("log line %q does not contain %q", logLine, want)
		}
	}
	if strings.Contains(logLine, cfg.Connection.APIKey) {
		t.Fatalf("log line contains API key: %q", logLine)
	}
}
