package cmd

import (
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run as a DNP3 outstation",
	Long: banner + `
The server role is designed to act like a DNP3 outstation.
In direct mode the server is a DNP3 outstation.
In inject mode the server should run on (or near) a legitimate DNP3 outstation.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
}
