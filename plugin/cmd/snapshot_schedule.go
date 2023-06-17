package cmd

import "github.com/spf13/cobra"

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage QuestDB snapshot schedules",
	Run: func(cmd *cobra.Command, args []string) {
		println("Snapshot Schedule")
	},
}
