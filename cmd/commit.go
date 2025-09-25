package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/MaiMarincic/bruh/config"
	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

var commitCmd = &cobra.Command{
	Use:   "commit [message]",
	Short: "Commit staged changes with an AI-generated commit message and pre-commit cleanup",
	Long: `Commit staged changes using Claude to generate a well-formed commit message.
If no message is provided, Claude will analyze the changes and create an appropriate commit message.
By default, runs pre-commit cleanup to fix any issues before committing.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateGitRepo(); err != nil {
			return fmt.Errorf("not in a git repository")
		}

		if !hasStagedChanges() {
			return fmt.Errorf("no staged changes to commit")
		}

		cleanupPreCommit, _ := cmd.Flags().GetBool("cleanup-pre-commit")

		if !cmd.Flags().Changed("cleanup-pre-commit") {
			cfg := config.Get()
			repoName, err := getRepoName()
			if err == nil {
				for _, project := range cfg.CleanupPreCommit {
					if project == repoName {
						cleanupPreCommit = true
						break
					}
				}
			}
		}

		if cleanupPreCommit {
			if err := runPreCommitCleanup(); err != nil {
				return fmt.Errorf("pre-commit cleanup failed: %v", err)
			}
		}

		var commitMessage string
		if len(args) > 0 {
			commitMessage = strings.Join(args, " ")
		} else {
			s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
			s.Suffix = " Generating commit message with Claude..."
			s.Start()

			message, err := generateCommitMessage()
			s.Stop()

			if err != nil {
				return fmt.Errorf("failed to generate commit message: %v", err)
			}
			commitMessage = message
		}

		if err := performCommit(commitMessage); err != nil {
			return fmt.Errorf("failed to commit: %v", err)
		}

		fmt.Printf("Successfully committed with message: %s\n", commitMessage)
		return nil
	},
}

func init() {
	commitCmd.Flags().BoolP("interactive", "i", false, "Use interactive mode (not recommended for automated usage)")
	commitCmd.Flags().Bool("cleanup-pre-commit", false, "Run pre-commit cleanup before committing")
	rootCmd.AddCommand(commitCmd)
}

func hasStagedChanges() bool {
	cmd := exec.Command("git", "diff", "--cached", "--exit-code")
	err := cmd.Run()
	return err != nil
}

func runPreCommitCleanup() error {
	maxAttempts := 5

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
		s.Suffix = fmt.Sprintf(" Running pre-commit (attempt %d)...", attempt)
		s.Start()

		cmd := exec.Command("pre-commit", "run", "--all-files")
		output, err := cmd.CombinedOutput()
		s.Stop()

		if err == nil {
			fmt.Println("Pre-commit checks passed!")
			return nil
		}

		fmt.Printf("Pre-commit issues found (attempt %d/%d):\n%s\n", attempt, maxAttempts, string(output))

		s = spinner.New(spinner.CharSets[11], 100*time.Millisecond)
		s.Suffix = " Fixing pre-commit issues with Claude..."
		s.Start()

		if err := fixPreCommitIssues(string(output)); err != nil {
			s.Stop()
			return fmt.Errorf("failed to fix pre-commit issues: %v", err)
		}
		s.Stop()

		addCmd := exec.Command("git", "add", ".")
		if err := addCmd.Run(); err != nil {
			return fmt.Errorf("failed to stage fixed files: %v", err)
		}
	}

	return fmt.Errorf("failed to fix pre-commit issues after %d attempts", maxAttempts)
}

func fixPreCommitIssues(preCommitOutput string) error {
	prompt := fmt.Sprintf(`Fix the following pre-commit issues in the current directory:

%s

Please analyze the errors and fix all the issues automatically. Only fix the issues, don't explain what you're doing.`, preCommitOutput)

	claudeCmd := exec.Command("claude", "--print", "--dangerously-skip-permissions", "--allowedTools", "Bash(*),Read(*),Edit(*),Glob(*),Grep(*),MultiEdit(*)", "--", prompt)
	claudeCmd.Stdin = strings.NewReader("")
	output, err := claudeCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to fix issues with Claude: %v\nOutput: %s", err, string(output))
	}

	return nil
}

func generateCommitMessage() (string, error) {
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusOutput, err := statusCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git status: %v", err)
	}

	diffCmd := exec.Command("git", "diff", "--cached", "--name-status")
	diffOutput, err := diffCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git diff: %v", err)
	}

	prompt := fmt.Sprintf(`Based on the following git changes, generate a concise, short, well-formed commit message following conventional commit format:

Git Status:
%s

Changed Files:
%s

Do not mention anything in the likes of written by AI.
Please provide only the commit message without any additional text or explanation.`,
		string(statusOutput), string(diffOutput))

	claudeCmd := exec.Command("claude", "--print", "--allowedTools", "Bash(git:*)", "--", prompt)
	output, err := claudeCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to generate commit message with Claude: %v", err)
	}

	message := strings.TrimSpace(string(output))
	if message == "" {
		return "", fmt.Errorf("Claude generated empty commit message")
	}

	return message, nil
}

func performCommit(message string) error {
	cmd := exec.Command("git", "commit", "-m", message, "--no-verify")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git commit failed: %s", output)
	}
	return nil
}

func getRepoName() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get repository root: %v", err)
	}
	repoRoot := strings.TrimSpace(string(output))
	return filepath.Base(repoRoot), nil
}
