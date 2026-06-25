package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v4"
)

const configFileName = "config.yaml"

var DefaultConfigPath = resolveDefaultConfigPath()

type writableFS interface {
	WriteFile(name string, data []byte, perm fs.FileMode) error
}

func resolveDefaultConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return ".hosthalla/config.yaml"
	}

	return filepath.Join(homeDir, ".hosthalla", "config.yaml")
}

func (a *AppConfig) SaveToFS(fsys fs.FS) error {
	if a == nil {
		return errors.New("config is nil")
	}
	a.ApplyDefaults()

	content, err := yaml.Marshal(a)
	if err != nil {
		return fmt.Errorf("marshal config to yaml: %w", err)
	}

	writer, ok := fsys.(writableFS)
	if !ok {
		return fmt.Errorf("filesystem is not writable: %T", fsys)
	}

	if err := writer.WriteFile(configFileName, content, 0o644); err != nil {
		return fmt.Errorf("write config file %q: %w", configFileName, err)
	}

	return nil
}

func (a *AppConfig) LoadFromFS(fsys fs.FS) error {
	if a == nil {
		return errors.New("config is nil")
	}

	content, err := fs.ReadFile(fsys, configFileName)
	if err != nil {
		return fmt.Errorf("read config file %q: %w", configFileName, err)
	}

	if err := yaml.Unmarshal(content, a); err != nil {
		return fmt.Errorf("unmarshal config file %q: %w", configFileName, err)
	}
	a.ApplyDefaults()

	return nil
}

func (a *AppConfig) SaveToPath(path string) error {
	if a == nil {
		return errors.New("config is nil")
	}
	a.ApplyDefaults()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory for %q: %w", path, err)
	}

	content, err := yaml.Marshal(a)
	if err != nil {
		return fmt.Errorf("marshal config to yaml: %w", err)
	}

	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write config file %q: %w", path, err)
	}

	return nil
}

func (a *AppConfig) LoadFromPath(path string) error {
	if a == nil {
		return errors.New("config is nil")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			template := AppConfig{
				WEB: WEBConfig{
					Host: "0.0.0.0",
					Port: 8080,
				},
				LogLevel: DefaultLogLevel,
			}

			if saveErr := template.SaveToPath(path); saveErr != nil {
				return fmt.Errorf("config file %q not found and template creation failed: %w", path, saveErr)
			}

			return fmt.Errorf("config file %q not found: created template, fill it and restart", path)
		}

		return fmt.Errorf("read config file %q: %w", path, err)
	}

	if err := yaml.Unmarshal(content, a); err != nil {
		return fmt.Errorf("unmarshal config file %q: %w", path, err)
	}
	a.ApplyDefaults()

	return nil
}
