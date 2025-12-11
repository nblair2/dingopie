package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Run as a DNP3 master",
	Long: banner + `
The client role is designed to act like a DNP3 master.
In direct mode the client is a DNP3 master.
In inject mode the client should run on (or near) a legitimate DNP3 master.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		printBanner(cmd, args)
		if cmd.HasSubCommands() {
			return
		}
		ip, _ := cmd.Flags().GetString("server-ip")
		if ip == "" {
			fmt.Println("Error: --server-ip is required")
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(clientCmd)
}
