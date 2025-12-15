package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config 对应 config.yaml。
type Config struct {
	Port      int        `mapstructure:"port" yaml:"port"`
	AuthToken string     `mapstructure:"auth_token" yaml:"auth_token"`
	Upstreams []Upstream `mapstructure:"upstreams" yaml:"upstreams"`
}

type Upstream struct {
	Name        string   `mapstructure:"name" yaml:"name"`
	Command     string   `mapstructure:"command" yaml:"command"`
	Args        []string `mapstructure:"args" yaml:"args"`
	Workdir     string   `mapstructure:"workdir" yaml:"workdir"`
	AutoApprove bool     `mapstructure:"auto_approve" yaml:"auto_approve"`
}

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./config.yaml"
	}
	return filepath.Join(home, ".config", "gomcp", "config.yaml")
}

// Load 读取配置，允许传入相对或绝对路径。
func Load(path string) (*Config, error) {
	cfg := &Config{}
	v := viper.New()
	v.SetConfigFile(expandPath(path))
	v.SetConfigType("yaml")

	// 默认值
	v.SetDefault("port", 8080)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	return cfg, nil
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	return path
}
