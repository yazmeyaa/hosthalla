package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"
)

const DefaultLogLevel = "warning"

type AppConfig struct {
	WEB      WEBConfig      `yaml:"web"`
	Database DatabaseConfig `yaml:"database"`
	Security SecurityConfig `yaml:"security"`
	LogLevel string         `yaml:"log_level"`
}
type WEBConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

type SecurityConfig struct {
	SecretEncryptionKey string `yaml:"secret_encryption_key"`
}

func NewDefaultAppConfig() AppConfig {
	return AppConfig{
		WEB: WEBConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			User:     "hosthalla",
			Password: "hosthalla",
			Database: "hosthalla",
		},
		Security: SecurityConfig{
			SecretEncryptionKey: mustGenerateSecretEncryptionKey(),
		},
		LogLevel: DefaultLogLevel,
	}
}

func (w WEBConfig) ListenAddress() string {
	return net.JoinHostPort(w.Host, strconv.Itoa(w.Port))
}

func (d DatabaseConfig) ConnectionString() string {
	var user *url.Userinfo
	if d.Password == "" {
		user = url.User(d.User)
	} else {
		user = url.UserPassword(d.User, d.Password)
	}

	return (&url.URL{
		Scheme: "postgres",
		User:   user,
		Host:   net.JoinHostPort(d.Host, strconv.Itoa(d.Port)),
		Path:   d.Database,
	}).String()
}

func (a *AppConfig) ApplyDefaults() {
	if strings.TrimSpace(a.LogLevel) == "" {
		a.LogLevel = DefaultLogLevel
	}
}

func (a AppConfig) SlogLevel() (slog.Level, error) {
	return ParseLogLevel(a.LogLevel)
}

func (a *AppConfig) ToYAML() ([]byte, error) {
	if a == nil {
		return nil, fmt.Errorf("config is nil")
	}

	a.ApplyDefaults()

	content, err := yaml.Marshal(a)
	if err != nil {
		return nil, fmt.Errorf("marshal config to yaml: %w", err)
	}

	return content, nil
}

func ParseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log_level %q: expected debug, info, warning, or error", raw)
	}
}

func (a AppConfig) SecretEncryptionKey() ([]byte, error) {
	encoded := strings.TrimSpace(a.Security.SecretEncryptionKey)
	if encoded == "" {
		return nil, fmt.Errorf("security.secret_encryption_key is required")
	}
	value, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode security.secret_encryption_key: %w", err)
	}
	if len(value) != 32 {
		return nil, fmt.Errorf("security.secret_encryption_key must decode to 32 bytes")
	}
	return value, nil
}

func mustGenerateSecretEncryptionKey() string {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(fmt.Sprintf("generate security.secret_encryption_key: %v", err))
	}
	return base64.StdEncoding.EncodeToString(key)
}
