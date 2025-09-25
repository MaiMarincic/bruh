package config

type Config struct {
	Branch BranchConfig `mapstructure:"branch"`
}

type BranchConfig struct {
	UsingTmux   bool   `mapstructure:"using_tmux"`
	Editor      string `mapstructure:"editor"`
}

func DefaultConfig() *Config {
	return &Config{
		Branch: BranchConfig{
			UsingTmux: true,
			Editor:    "nvim",
		},
	}
}