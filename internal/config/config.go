package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the runtime configuration loaded from YAML.
type Config struct {
	Port      int        `yaml:"port"`
	AuthToken string     `yaml:"auth_token"`
	Upstreams []Upstream `yaml:"upstreams"`
}

// Upstream describes a single MCP server that will be launched via stdio.
type Upstream struct {
	Name        string   `yaml:"name"`
	Command     string   `yaml:"command"`
	Args        []string `yaml:"args"`
	Workdir     string   `yaml:"workdir"`
	Env         []string `yaml:"env"`
	AutoApprove bool     `yaml:"auto_approve"`
}

// DefaultPath returns the default config file path under ~/.config/gomcp/config.yaml.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./config.yaml"
	}
	return filepath.Join(home, ".config", "gomcp", "config.yaml")
}

// Load reads and unmarshals a YAML config file.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Port == 0 {
		c.Port = 8080
	}
	if len(c.Upstreams) == 0 {
		return errors.New("no upstreams configured")
	}
	for _, ups := range c.Upstreams {
		if ups.Name == "" {
			return fmt.Errorf("upstream missing name")
		}
		if ups.Command == "" {
			return fmt.Errorf("upstream %s missing command", ups.Name)
		}
	}
	return nil
}
