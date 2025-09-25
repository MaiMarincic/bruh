package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

var globalConfig *Config

func Load() (*Config, error) {
	if globalConfig != nil {
		return globalConfig, nil
	}

	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")

	configDir := getConfigDir()
	v.AddConfigPath(configDir)
	v.AddConfigPath(".")

	v.SetEnvPrefix("BRUH")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	defaults := DefaultConfig()
	v.SetDefault("branch.using_tmux", defaults.Branch.UsingTmux)
	v.SetDefault("branch.editor", defaults.Branch.Editor)
	v.SetDefault("pr.prompts", defaults.PR.Prompts)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, err
	}

	globalConfig = &config
	return globalConfig, nil
}

func Get() *Config {
	if globalConfig == nil {
		config, err := Load()
		if err != nil {
			return DefaultConfig()
		}
		return config
	}
	return globalConfig
}

func getConfigDir() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "bruh")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}

	return filepath.Join(home, ".config", "bruh")
}