package cmd

import (
	"fmt"
	"os"

	"github.com/MaiMarincic/bruh/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "bruh",
	Short: "Just usefull commands",
	Long:  "Just usefull commands",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		_, err := config.Load()
		return err
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(branchCmd)
	rootCmd.AddCommand(prCmd)
}
