package cmd

import "github.com/spf13/cobra"

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Take a snapshot of a QuestDB instance",
	Run: func(cmd *cobra.Command, args []string) {
		println("Snapshot")
	},
}
