package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var addcheatCmd = &cobra.Command{
	Use:   "addcheat",
	Short: "Add the last command from history to a navi cheat sheet",
	Long:  "Retrieves the last command from shell history and asks Claude Code to add it to an appropriate navi cheat sheet",
	RunE: func(cmd *cobra.Command, args []string) error {
		cheatDir, _ := cmd.Flags().GetString("cheat-directory")
		if cheatDir == "" {
			return fmt.Errorf("--cheat-directory flag is required")
		}

		absCheatDir, err := filepath.Abs(cheatDir)
		if err != nil {
			return fmt.Errorf("invalid cheat directory path: %v", err)
		}

		if _, err := os.Stat(absCheatDir); os.IsNotExist(err) {
			return fmt.Errorf("cheat directory does not exist: %s", absCheatDir)
		}

		lastCommand, err := getLastCommand()
		if err != nil {
			return fmt.Errorf("failed to get last command: %v", err)
		}

		if lastCommand == "" {
			return fmt.Errorf("no command found in history")
		}

		if err := sendToClaudeCode(lastCommand, absCheatDir); err != nil {
			return fmt.Errorf("failed to send to Claude Code: %v", err)
		}

		fmt.Printf("Sent command to Claude Code for addition to cheat sheets\n")
		return nil
	},
}

func init() {
	addcheatCmd.Flags().StringP("cheat-directory", "d", "", "Directory containing navi cheat sheets")
	addcheatCmd.MarkFlagRequired("cheat-directory")
}

func getLastCommand() (string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	var histCmd *exec.Cmd
	if strings.Contains(shell, "zsh") {
		histCmd = exec.Command("zsh", "-ic", "fc -ln -1")
	} else if strings.Contains(shell, "bash") {
		histCmd = exec.Command("bash", "-ic", "history | tail -2 | head -1 | sed 's/^[ ]*[0-9]*[ ]*//'")
	} else if strings.Contains(shell, "fish") {
		histCmd = exec.Command("fish", "-c", "history | head -1")
	} else {
		histCmd = exec.Command("sh", "-c", "fc -ln -1")
	}

	output, err := histCmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

func sendToClaudeCode(command, cheatDir string) error {
	prompt := fmt.Sprintf(`Add this command to the appropriate navi cheat sheet in the directory %s:

Command: %s

Instructions:
1. Find the most appropriate existing cheat sheet file (.cheat) in the directory
2. If no appropriate file exists, create a new one with a suitable name
3. Add the command with proper navi syntax, including:
   - A descriptive comment (starting with #)
   - The command itself (starting with $)
   - Any relevant tags or variables if the command has parameters
4. Ensure the formatting follows navi conventions

Make the addition concise and useful for future reference.`, cheatDir, command)

	claudeCmd := exec.Command("claude", prompt, cheatDir)
	claudeCmd.Stdout = os.Stdout
	claudeCmd.Stderr = os.Stderr
	claudeCmd.Stdin = os.Stdin

	return claudeCmd.Run()
}