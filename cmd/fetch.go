package cmd

import (
	"github.com/spf13/cobra"
)

// fetchCmd represents the fetch command
var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Queue up a fetch manually",
	Long:  `Queue up a fetch manually.`,
}

func init() {
	rootCmd.AddCommand(fetchCmd)
}
