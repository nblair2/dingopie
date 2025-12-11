package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var clientInjectCmd = &cobra.Command{
	Use:   "inject",
	Short: "Run client in inject mode",
	Long: banner + `
In inject mode, dingopie rides on top of an existing DNP3 channel.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var clientInjectSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send data from client to server",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("client inject send called - not implemented")
	},
}

var clientInjectReceiveCmd = &cobra.Command{
	Use:   "receive",
	Short: "Receive data from server",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("client inject receive called - not implemented")
	},
}

var clientInjectShellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Tunnel a shell",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("client inject shell called - not implemented")
	},
}

var clientInjectConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to a shell",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("client inject connect called - not implemented")
	},
}

func init() {
	clientCmd.AddCommand(clientInjectCmd)
	clientInjectCmd.AddCommand(clientInjectSendCmd)
	clientInjectCmd.AddCommand(clientInjectReceiveCmd)
	clientInjectCmd.AddCommand(clientInjectShellCmd)
	clientInjectCmd.AddCommand(clientInjectConnectCmd)

	clientInjectSendCmd.Flags().
		StringP("file", "f", "", "file to read data from (default is command line)")
	clientInjectReceiveCmd.Flags().
		StringP("file", "f", "", "file to write data to (default is to stdout)")
}
