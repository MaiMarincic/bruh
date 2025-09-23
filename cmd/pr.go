package cmd

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

var prCmd = &cobra.Command{
	Use:   "pr create",
	Short: "Create a pull request with AI-generated summary and test criteria",
	Long: `Create a pull request using GitHub CLI with Claude-generated summary and testing criteria.
Claude will analyze the changes between the current branch and the base branch to create
a comprehensive PR description.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate git repository
		if err := validateGitRepo(); err != nil {
			return fmt.Errorf("not in a git repository")
		}

		// Check if we're on a branch other than main/master
		currentBranch, err := getCurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %v", err)
		}

		if currentBranch == "main" || currentBranch == "master" {
			return fmt.Errorf("cannot create PR from %s branch", currentBranch)
		}

		// Check if gh CLI is available
		if err := checkGHCLI(); err != nil {
			return fmt.Errorf("GitHub CLI (gh) is not installed or not authenticated: %v", err)
		}

		// Generate PR description using Claude with spinner
		s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
		s.Suffix = " Analyzing changes and generating PR description with Claude..."
		s.Start()

		prDescription, err := generatePRDescription(currentBranch)
		s.Stop()

		if err != nil {
			return fmt.Errorf("failed to generate PR description: %v", err)
		}

		fmt.Printf("Generated PR description:\n%s\n\n", prDescription)

		// Create the PR using gh CLI
		s = spinner.New(spinner.CharSets[11], 100*time.Millisecond)
		s.Suffix = " Creating pull request..."
		s.Start()

		prURL, err := createPR(prDescription)
		s.Stop()

		if err != nil {
			return fmt.Errorf("failed to create PR: %v", err)
		}

		fmt.Printf("Successfully created pull request: %s\n", prURL)
		return nil
	},
}

func init() {
	prCmd.Flags().StringP("base", "b", "", "Base branch for the PR (defaults to repository default)")
	prCmd.Flags().StringP("title", "t", "", "PR title (Claude will generate if not provided)")
}

func getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func checkGHCLI() error {
	// Check if gh is installed
	cmd := exec.Command("gh", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh CLI not found")
	}

	// Check if gh is authenticated
	cmd = exec.Command("gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh CLI not authenticated")
	}

	return nil
}

func generatePRDescription(currentBranch string) (string, error) {
	// Get the base branch (default branch of the repository)
	baseCmd := exec.Command("gh", "repo", "view", "--json", "defaultBranchRef", "--jq", ".defaultBranchRef.name")
	baseOutput, err := baseCmd.Output()
	var baseBranch string
	if err != nil {
		// Fall back to main/master if we can't get the default branch
		baseBranch = "main"
	} else {
		baseBranch = strings.TrimSpace(string(baseOutput))
	}

	// Get the diff between current branch and base
	diffCmd := exec.Command("git", "diff", fmt.Sprintf("%s...HEAD", baseBranch), "--name-status")
	diffOutput, err := diffCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get diff: %v", err)
	}

	// Get commit messages
	logCmd := exec.Command("git", "log", fmt.Sprintf("%s..HEAD", baseBranch), "--oneline")
	logOutput, err := logCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get commit log: %v", err)
	}

	// Get detailed diff for context
	detailedDiffCmd := exec.Command("git", "diff", fmt.Sprintf("%s...HEAD", baseBranch), "--stat")
	detailedDiffOutput, err := detailedDiffCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get detailed diff: %v", err)
	}

	// Use Claude to analyze changes and generate PR description
	prompt := fmt.Sprintf(`Based on the following git changes between %s and %s branches, create a pull request using the gh CLI.

Changed Files:
%s

Commit History:
%s

Detailed Changes:
%s

Please use the gh pr create command with the following requirements:
1. Generate a concise, descriptive PR title
2. Create a comprehensive PR body that includes:
   - A summary section with 2-3 bullet points explaining what this PR does
   - A test plan section with specific testing criteria and checklist items
3. Use the --allowedTools flag to enable the gh tool
4. The PR body should be well-formatted with markdown
5. Include "🤖 Generated with Claude Code" at the end of the body

Important: Execute the gh pr create command directly. Do not just return the command or description text.`,
		currentBranch, baseBranch,
		string(diffOutput),
		string(logOutput),
		string(detailedDiffOutput))

	claudeCmd := exec.Command("claude", "--print", "--allowedTools", "Bash(gh:*)", "--", prompt)
	output, err := claudeCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to create PR with Claude: %v", err)
	}

	return strings.TrimSpace(string(output)), nil
}

func createPR(description string) (string, error) {
	// The description from Claude should already contain the PR URL from the gh pr create command
	// We just need to extract it
	lines := strings.Split(description, "\n")
	for _, line := range lines {
		if strings.Contains(line, "github.com") && strings.Contains(line, "/pull/") {
			return strings.TrimSpace(line), nil
		}
	}

	// If we couldn't find a URL in the output, return the full output
	return description, nil
}
