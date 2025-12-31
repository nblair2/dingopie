package cmd

import (
	"dingopie/internal"
	"dingopie/internal/primary"
	"dingopie/internal/secondary"
	"dingopie/internal/shell"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run as a DNP3 outstation",
	Long: internal.Banner + `
The server role is designed to act like a DNP3 outstation.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var serverDirectCmd = &cobra.Command{
	Use:   "direct",
	Short: "Run server in direct mode",
	Long: internal.Banner + `
In direct mode, dingopie creates a new DNP3 channel.
Data is sent in DNP3 Application Objects.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var serverDirectSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send data from server to client",
	Run: func(cmd *cobra.Command, args []string) {
		ip, _ := cmd.Flags().GetString("server-ip")
		port, _ := cmd.Flags().GetInt("server-port")
		file, _ := cmd.Flags().GetString("file")
		key, _ := cmd.Flags().GetString("key")
		objs, _ := cmd.Flags().GetInt("objects")

		if objs > 60 {
			fmt.Println("Error: objects cannot be greater than 60")

			return
		}

		fmt.Println(">> Parameters:")
		if ip != "" {
			fmt.Printf(">>>> Server IP: %s\n", ip)
		}
		fmt.Printf(">>>> Server Port: %d\n", port)
		fmt.Printf(">>>> Num Objects: %d (x4 = %d bytes/message)\n", objs, objs*4)

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

		err = secondary.ServerSend(ip, port, data, objs)
		if err != nil {
			fmt.Printf("Error with direct send: %v\n", err)
			os.Exit(1)
		}
	},
}

var serverDirectReceiveCmd = &cobra.Command{
	Use:   "receive",
	Short: "Receive data from client",
	Run: func(cmd *cobra.Command, args []string) {
		ip, _ := cmd.Flags().GetString("server-ip")
		port, _ := cmd.Flags().GetInt("server-port")
		file, _ := cmd.Flags().GetString("file")
		key, _ := cmd.Flags().GetString("key")

		fmt.Println(">> Parameters:")
		if ip != "" {
			fmt.Printf(">>>> Server IP: %s\n", ip)
		}
		fmt.Printf(">>>> Server Port: %d\n", port)

		data, err := primary.ServerReceive(ip, port)
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

var serverDirectShellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Tunnel a shell",
	Run: func(cmd *cobra.Command, args []string) {
		ip, _ := cmd.Flags().GetString("server-ip")
		port, _ := cmd.Flags().GetInt("server-port")
		command, _ := cmd.Flags().GetString("command")
		key, _ := cmd.Flags().GetString("key")

		fmt.Println(">> Parameters:")
		if ip != "" {
			fmt.Printf(">>>> Server IP: %s\n", ip)
		}
		fmt.Printf(">>>> Server Port: %d\n", port)
		if command != "" {
			fmt.Printf(">>>> Command: %s\n", command)
		} else {
			command = os.Getenv("SHELL")
		}
		if key != "" {
			fmt.Printf(">>>> Key: %s\n", key)
		}
		err := shell.ServerShell(command, key, ip, port)
		if err != nil {
			fmt.Printf("Error opening shell: %v\n", err)
		}
	},
}

var serverDirectConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to a shell",
	Run: func(cmd *cobra.Command, args []string) {
		ip, _ := cmd.Flags().GetString("server-ip")
		port, _ := cmd.Flags().GetInt("server-port")
		key, _ := cmd.Flags().GetString("key")

		fmt.Println(">> Parameters:")
		if ip != "" {
			fmt.Printf(">>>> Server IP: %s\n", ip)
		}
		fmt.Printf(">>>> Server Port: %d\n", port)
		if key != "" {
			fmt.Printf(">>>> Key: %s\n", key)
		}

		err := shell.ServerConnect(key, ip, port)
		if err != nil {
			fmt.Printf("Error connecting to shell: %v\n", err)
		}
		fmt.Println(">> Shell session ended")
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.AddCommand(serverDirectCmd)
	serverDirectCmd.AddCommand(serverDirectSendCmd)
	serverDirectCmd.AddCommand(serverDirectReceiveCmd)
	serverDirectCmd.AddCommand(serverDirectShellCmd)
	serverDirectCmd.AddCommand(serverDirectConnectCmd)
	serverDirectCmd.AddCommand(serverDirectConnectCmd)

	serverDirectSendCmd.Flags().
		StringP("file", "f", "", "file to read data from (default is command line)")
	serverDirectSendCmd.Flags().
		IntP("objects", "o", 8, "number of 4-byte objects to send in each message (max 60)")
	serverDirectReceiveCmd.Flags().
		StringP("file", "f", "", "file to write data to (default is to stdout)")
	serverDirectShellCmd.Flags().
		StringP("command", "c", "", "command to start an interactive shell (default is $SHELL)")
}
