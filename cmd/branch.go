package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type Config struct {
	BranchName   string
	TmuxMode     string
	WorktreePath string
	SessionName  string
}

var branchCmd = &cobra.Command{
	Use:   "branch [branch-name]",
	Short: "Create a git worktree for a branch and open it in tmux",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		config := Config{
			BranchName: args[0],
		}

		tmuxMode, _ := cmd.Flags().GetString("tmux")
		config.TmuxMode = tmuxMode

		worktreePath, _ := cmd.Flags().GetString("path")
		config.WorktreePath = worktreePath

		sessionName, _ := cmd.Flags().GetString("session")
		config.SessionName = sessionName

		if config.TmuxMode != "window" && config.TmuxMode != "session" {
			return fmt.Errorf("tmux mode must be 'window' or 'session'")
		}

		if err := validateGitRepo(); err != nil {
			return fmt.Errorf("not in a git repository")
		}

		worktreePath, err := createWorktree(config)
		if err != nil {
			return fmt.Errorf("error creating worktree: %v", err)
		}

		if err := openInTmux(worktreePath, config); err != nil {
			return fmt.Errorf("error opening tmux: %v", err)
		}

		return nil
	},
}

func init() {
	branchCmd.Flags().StringP("tmux", "t", "window", "Tmux mode: 'window' or 'session'")
	branchCmd.Flags().StringP("path", "p", "", "Custom path for the worktree (optional)")
	branchCmd.Flags().StringP("session", "s", "", "Tmux session name (only for session mode)")
}

func validateGitRepo() error {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("not in a git repository")
	}
	return nil
}

func createWorktree(config Config) (string, error) {
	worktreePath := config.WorktreePath
	if worktreePath == "" {
		repoRoot, err := getRepoRoot()
		if err != nil {
			return "", err
		}
		parentDir := filepath.Dir(repoRoot)
		repoName := filepath.Base(repoRoot)
		worktreePath = filepath.Join(parentDir, fmt.Sprintf("%s-%s", repoName, config.BranchName))
	}

	// Check if branch exists locally or on remote
	localExists := branchExistsLocally(config.BranchName)
	remoteRef := findRemoteBranch(config.BranchName)

	var cmd *exec.Cmd
	if localExists {
		// Use existing local branch
		cmd = exec.Command("git", "worktree", "add", worktreePath, config.BranchName)
	} else if remoteRef != "" {
		// Create worktree from remote branch
		cmd = exec.Command("git", "worktree", "add", worktreePath, remoteRef)
	} else {
		// Create new local branch
		cmd = exec.Command("git", "worktree", "add", worktreePath, "-b", config.BranchName)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to create worktree: %s", output)
	}

	fmt.Printf("Created worktree at: %s\n", worktreePath)
	return worktreePath, nil
}

func getRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get repository root: %v", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func branchExistsLocally(branchName string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", branchName)
	err := cmd.Run()
	return err == nil
}

func findRemoteBranch(branchName string) string {
	// First try to find exact match on any remote
	cmd := exec.Command("git", "branch", "-r", "--list", fmt.Sprintf("*/%s", branchName))
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.Contains(line, "->") {
			return line
		}
	}

	return ""
}

func openInTmux(worktreePath string, config Config) error {
	if !isTmuxRunning() {
		return fmt.Errorf("tmux is not running")
	}

	var target string
	if config.TmuxMode == "session" {
		sessionName := config.SessionName
		if sessionName == "" {
			sessionName = filepath.Base(worktreePath)
		}
		cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-c", worktreePath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to create tmux session: %v", err)
		}
		target = sessionName

		cmd = exec.Command("tmux", "switch-client", "-t", sessionName)
		if err := cmd.Run(); err != nil {
			cmd = exec.Command("tmux", "attach-session", "-t", sessionName)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to switch to tmux session: %v", err)
			}
		}
	} else {
		cmd := exec.Command("tmux", "new-window", "-c", worktreePath, "-n", filepath.Base(worktreePath))
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to create tmux window: %v", err)
		}
		target = strings.TrimSpace(string(output))
		if target == "" {
			target = fmt.Sprintf(":%s", filepath.Base(worktreePath))
		}
	}

	if err := openClaudeCodeInTmux(worktreePath, target); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to open Claude Code: %v\n", err)
	}

	fmt.Printf("Opened tmux %s at: %s\n", config.TmuxMode, worktreePath)
	return nil
}

func isTmuxRunning() bool {
	return os.Getenv("TMUX") != ""
}

func openClaudeCodeInTmux(worktreePath, target string) error {
	cmd := exec.Command("tmux", "send-keys", "-t", target, fmt.Sprintf("claude %s", worktreePath), "Enter")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to send Claude command to tmux: %v", err)
	}

	fmt.Printf("Opened Claude at: %s\n", worktreePath)
	return nil
}
