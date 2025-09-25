package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/MaiMarincic/bruh/config"
	"github.com/spf13/cobra"
)

type BranchRuntime struct {
	UsingTmux  bool
	FromBranch string
	BranchName string
	Editor     string
}

var branchCmd = &cobra.Command{
	Use:   "branch",
	Short: "Create a git worktree and open editor",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Get()

		usingTmux, _ := cmd.Flags().GetBool("using-tmux")
		if !cmd.Flags().Changed("using-tmux") {
			usingTmux = cfg.Branch.UsingTmux
		}

		fromBranch, _ := cmd.Flags().GetString("from-branch")
		if fromBranch == "" {
			currentBranch, err := getCurrentBranch()
			if err != nil {
				return fmt.Errorf("failed to get current branch: %v", err)
			}
			fromBranch = currentBranch
		}

		branchName, _ := cmd.Flags().GetString("branch-name")
		if branchName == "" {
			branchName = fromBranch + "-worktree"
		}

		editor, _ := cmd.Flags().GetString("editor")
		if editor == "" {
			editor = cfg.Branch.Editor
		}

		runtime := BranchRuntime{
			UsingTmux:  usingTmux,
			FromBranch: fromBranch,
			BranchName: branchName,
			Editor:     editor,
		}

		if err := validateGitRepo(); err != nil {
			return fmt.Errorf("not in a git repository")
		}

		worktreePath, err := createWorktree(runtime)
		if err != nil {
			return fmt.Errorf("error creating worktree: %v", err)
		}

		if err := openEditor(worktreePath, runtime); err != nil {
			return fmt.Errorf("error opening editor: %v", err)
		}

		return nil
	},
}

func init() {
	branchCmd.Flags().Bool("using-tmux", false, "Use tmux for editor session (default from config)")
	branchCmd.Flags().String("from-branch", "", "Branch from which to create worktree (default: current branch)")
	branchCmd.Flags().String("branch-name", "", "Name of worktree branch (default: <from-branch>-worktree)")
	branchCmd.Flags().String("editor", "", "Editor to open (default from config)")
}

func validateGitRepo() error {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("not in a git repository")
	}
	return nil
}

func getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %v", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func createWorktree(runtime BranchRuntime) (string, error) {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return "", err
	}

	parentDir := filepath.Dir(repoRoot)
	repoName := filepath.Base(repoRoot)
	worktreePath := filepath.Join(parentDir, fmt.Sprintf("%s-%s", repoName, runtime.BranchName))

	cmd := exec.Command("git", "worktree", "add", worktreePath, "-b", runtime.BranchName, runtime.FromBranch)
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

func openEditor(worktreePath string, runtime BranchRuntime) error {
	if runtime.UsingTmux && isTmuxRunning() {
		return openEditorInTmux(worktreePath, runtime)
	}
	return openEditorDirect(worktreePath, runtime)
}

func openEditorInTmux(worktreePath string, runtime BranchRuntime) error {
	cmd := exec.Command("tmux", "new-window", "-c", worktreePath, "-n", filepath.Base(worktreePath))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create tmux window: %v", err)
	}

	cmd = exec.Command("tmux", "send-keys", "-t", fmt.Sprintf(":%s", filepath.Base(worktreePath)), runtime.Editor, "Enter")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open editor in tmux: %v", err)
	}

	fmt.Printf("Opened %s in tmux window at: %s\n", runtime.Editor, worktreePath)
	return nil
}

func openEditorDirect(worktreePath string, runtime BranchRuntime) error {
	cmd := exec.Command(runtime.Editor, worktreePath)
	cmd.Dir = worktreePath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open editor: %v", err)
	}

	fmt.Printf("Opened %s at: %s\n", runtime.Editor, worktreePath)
	return nil
}

func isTmuxRunning() bool {
	return os.Getenv("TMUX") != ""
}
