package cmd

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

var commitCmd = &cobra.Command{
	Use:   "commit [message]",
	Short: "Commit staged changes with an AI-generated commit message",
	Long: `Commit staged changes using Claude to generate a well-formed commit message.
If no message is provided, Claude will analyze the changes and create an appropriate commit message.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate git repository
		if err := validateGitRepo(); err != nil {
			return fmt.Errorf("not in a git repository")
		}

		// Check if there are staged changes
		if !hasStagedChanges() {
			return fmt.Errorf("no staged changes to commit")
		}

		var commitMessage string
		if len(args) > 0 {
			// Use provided message
			commitMessage = strings.Join(args, " ")
		} else {
			// Generate commit message using Claude with spinner
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

		// Perform the commit
		if err := performCommit(commitMessage); err != nil {
			return fmt.Errorf("failed to commit: %v", err)
		}

		fmt.Printf("Successfully committed with message: %s\n", commitMessage)
		return nil
	},
}

func init() {
	commitCmd.Flags().BoolP("interactive", "i", false, "Use interactive mode (not recommended for automated usage)")
}

func hasStagedChanges() bool {
	cmd := exec.Command("git", "diff", "--cached", "--exit-code")
	err := cmd.Run()
	// Exit code 1 means there are differences (staged changes)
	// Exit code 0 means no differences (no staged changes)
	return err != nil
}

func generateCommitMessage() (string, error) {
	// Get git status and diff information
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

	// Use Claude to analyze changes and generate commit message
	prompt := fmt.Sprintf(`Based on the following git changes, generate a concise, well-formed commit message following conventional commit format:

Git Status:
%s

Changed Files:
%s

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
