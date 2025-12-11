// Package cmd implements the command line interface for dingopie
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// serverInjectCmd represents the inject mode for server.
var serverInjectCmd = &cobra.Command{
	Use:   "inject",
	Short: "Run server in inject mode",
	Long: banner + `
In inject mode, dingopie rides on top of an existing DNP3 channel.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var serverInjectSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send data from server to client",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("server inject send called - not implemented")
	},
}

var serverInjectReceiveCmd = &cobra.Command{
	Use:   "receive",
	Short: "Receive data from client",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("server inject receive called - not implemented")
	},
}

var serverInjectShellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Tunnel a shell",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("server inject shell called - not implemented")
	},
}

var serverInjectConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to a shell",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("server inject connect called - not implemented")
	},
}

func init() {
	serverCmd.AddCommand(serverInjectCmd)
	serverInjectCmd.AddCommand(serverInjectSendCmd)
	serverInjectCmd.AddCommand(serverInjectReceiveCmd)
	serverInjectCmd.AddCommand(serverInjectShellCmd)
	serverInjectCmd.AddCommand(serverInjectConnectCmd)

	serverInjectSendCmd.Flags().
		StringP("file", "f", "", "file to read data from (default is command line)")
	serverInjectReceiveCmd.Flags().
		StringP("file", "f", "", "file to write data to (default is to stdout)")
}
