package workspace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const (
	ConfigFileName = ".wsx.json"
	EnvFileName    = ".wsx.env"
)

var ErrWorkspaceNotFound = errors.New("workspace not found")

type Config struct {
	Version string    `json:"version"`
	Name    string    `json:"name"`
	Created time.Time `json:"created"`
	Refs    []Ref     `json:"refs"`
}

type Ref struct {
	Name  string    `json:"name"`
	Path  string    `json:"path"`
	Added time.Time `json:"added"`
}

type LoadedConfig struct {
	Root       string
	ConfigPath string
	Config     Config
}

func FindWorkspaceRoot(startDir string) (string, error) {
	if startDir == "" {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	current, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		configPath := filepath.Join(current, ConfigFileName)
		if _, err := os.Stat(configPath); err == nil {
			return current, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", ErrWorkspaceNotFound
		}
		current = parent
	}
}

func LoadConfig(startDir string) (*LoadedConfig, error) {
	root, err := FindWorkspaceRoot(startDir)
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(root, ConfigFileName)
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}

	if cfg.Refs == nil {
		cfg.Refs = []Ref{}
	}

	return &LoadedConfig{
		Root:       root,
		ConfigPath: configPath,
		Config:     cfg,
	}, nil
}

func SaveConfig(root string, cfg Config) error {
	if cfg.Refs == nil {
		cfg.Refs = []Ref{}
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(filepath.Join(root, ConfigFileName), data, 0o644)
}
