package cmd

import "github.com/spf13/cobra"

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore a QuestDB instance from a snapshot",
	Run: func(cmd *cobra.Command, args []string) {
		println("Restore")
	},
}
