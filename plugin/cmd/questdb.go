package cmd

import (
	"github.com/spf13/cobra"
)

var questdbCommand = &cobra.Command{
	Use:   "questdb",
	Short: "Manage QuestDB instances",
	Run: func(cmd *cobra.Command, args []string) {
		println("QuestDB")
	},
}
