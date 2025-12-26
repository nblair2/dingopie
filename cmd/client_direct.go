package cmd

import (
	"dingopie/internal"
	"dingopie/internal/primary"
	"dingopie/internal/secondary"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var clientDirectCmd = &cobra.Command{
	Use:   "direct",
	Short: "Run client in direct mode",
	Long: banner + `
In direct mode, dingopie creates a new DNP3 channel. Data is sent in DNP3 Application Objects.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var clientDirectSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send data from client to server",
	Run: func(cmd *cobra.Command, args []string) {
		ip, _ := cmd.Flags().GetString("server-ip")
		port, _ := cmd.Flags().GetInt("server-port")
		file, _ := cmd.Flags().GetString("file")
		wait, _ := cmd.Flags().GetDuration("wait")
		key, _ := cmd.Flags().GetString("key")
		objects, _ := cmd.Flags().GetInt("objects")

		fmt.Println(">> Parameters:")
		fmt.Printf(">>>> Server IP: %s\n", ip)
		fmt.Printf(">>>> Server Port: %d\n", port)
		fmt.Printf(">>>> Wait: %v\n", wait)
		fmt.Printf(">>>> Num Objects: %d (x4 = %d bytes/message)\n", objects, objects*4)

		var data []byte
		var err error

		if file != "" {
			//nolint:gosec // G304: file is provided by user, needs permissions to access
			data, err = os.ReadFile(file)
			if err != nil {
				fmt.Printf("Error reading file: %v\n", err)

				return
			}
			fmt.Printf(">>>> File: %s\n", file)
		} else if len(args) > 0 {
			data = []byte(args[0])
		} else {
			fmt.Println("No data provided to send")

			return
		}

		if key != "" {
			fmt.Printf(">>>> Key: %s\n", key)
			data = internal.XorData(key, data)
		}

		err = primary.ClientSend(ip, port, data, wait, objects)
		if err != nil {
			fmt.Printf(
				"Error with direct send: %v", err)
			os.Exit(1)
		}
	},
}

var clientDirectReceiveCmd = &cobra.Command{
	Use:   "receive",
	Short: "Receive data from server",
	Run: func(cmd *cobra.Command, args []string) {
		ip, _ := cmd.Flags().GetString("server-ip")
		port, _ := cmd.Flags().GetInt("server-port")
		file, _ := cmd.Flags().GetString("file")
		wait, _ := cmd.Flags().GetDuration("wait")
		key, _ := cmd.Flags().GetString("key")

		fmt.Println(">> Parameters:")
		fmt.Printf(">>>> Server IP: %s\n", ip)
		fmt.Printf(">>>> Server Port: %d\n", port)
		fmt.Printf(">>>> Wait: %v\n", wait)

		if file != "" {
			_, err := os.Stat(file)
			if err == nil {
				fmt.Printf("Error: file %s already exists\n", file)

				return
			}
			fmt.Printf(">>>> File: %s\n", file)
		}

		data, err := secondary.ClientReceive(ip, port, wait)
		if err != nil {
			fmt.Printf(
				"Error with direct receive: %v\nAttempting to output what data we have\n",
				err,
			)
		}

		if key != "" {
			fmt.Println(">> Decrypting data")
			fmt.Printf(">>>> Key: %s\n", key)
			data = internal.XorData(key, data)
		}

		if file != "" {
			err := os.WriteFile(file, data, 0o400)
			if err != nil {
				fmt.Printf("Error writing to file: %v\n", err)
			} else {
				fmt.Printf(">> Data written to %s\n", file)
			}
		} else {
			fmt.Printf(">> Message: %s\n", string(data))
		}
	},
}

var clientDirectShellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Tunnel a shell",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("client direct shell called - not implemented")
	},
}

var clientDirectConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to a shell",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("client direct connect called - not implemented")
	},
}

func init() {
	clientCmd.AddCommand(clientDirectCmd)
	clientDirectCmd.AddCommand(clientDirectSendCmd)
	clientDirectCmd.AddCommand(clientDirectReceiveCmd)
	clientDirectCmd.AddCommand(clientDirectShellCmd)
	clientDirectCmd.AddCommand(clientDirectConnectCmd)
	clientDirectCmd.PersistentFlags().
		DurationP("wait", "w", 1*time.Second, "wait time between DNP3 messages")

	clientDirectSendCmd.Flags().
		StringP("file", "f", "", "file to read data from (default is a positional argument)")
	clientDirectSendCmd.Flags().
		IntP("size", "s", 32, "number of bytes to send in each message")
	clientDirectReceiveCmd.Flags().
		StringP("file", "f", "", "file to write data to (default is to stdout)")

	clientDirectSendCmd.Flags().
		IntP("objects", "o", 8, "number of 4-byte objects to send in each message (max 48)")
}
