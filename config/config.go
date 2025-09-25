package config

type Config struct {
	Branch           BranchConfig `mapstructure:"branch"`
	PR               PRConfig     `mapstructure:"pr"`
	CleanupPreCommit []string     `mapstructure:"cleanup-pre-commit"`
}

type BranchConfig struct {
	UsingTmux   bool   `mapstructure:"using_tmux"`
	Editor      string `mapstructure:"editor"`
}

type PRConfig struct {
	Prompts map[string]string `mapstructure:"prompts"`
}

func DefaultConfig() *Config {
	return &Config{
		Branch: BranchConfig{
			UsingTmux: true,
			Editor:    "nvim",
		},
		PR: PRConfig{
			Prompts: map[string]string{
				"default": "Based on the following git changes, create a PR. Execute the gh pr create command directly.",
			},
		},
		CleanupPreCommit: []string{},
	}
}