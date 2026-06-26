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
var ErrConfigAlreadyExists = errors.New("config file already exists")

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

	content, err := a.ToYAML()
	if err != nil {
		return err
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

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory for %q: %w", path, err)
	}

	content, err := a.ToYAML()
	if err != nil {
		return err
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
		return fmt.Errorf("read config file %q: %w", path, err)
	}

	if err := yaml.Unmarshal(content, a); err != nil {
		return fmt.Errorf("unmarshal config file %q: %w", path, err)
	}
	a.ApplyDefaults()

	return nil
}

func ReadYAMLFromPath(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %q: %w", path, err)
	}

	return content, nil
}

func GenerateDefaultConfig(path string, overwrite bool) error {
	if path == "" {
		return errors.New("config path is empty")
	}

	if !overwrite {
		exists, err := ConfigExists(path)
		if err != nil {
			return err
		}

		if exists {
			return fmt.Errorf("%w: %q", ErrConfigAlreadyExists, path)
		}
	}

	cfg := NewDefaultAppConfig()
	if err := cfg.SaveToPath(path); err != nil {
		return fmt.Errorf("create default config file %q: %w", path, err)
	}

	return nil
}

func ConfigExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return false, fmt.Errorf("check config file %q: %w", path, err)
}
