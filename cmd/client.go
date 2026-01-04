package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/nblair2/dingopie/internal"
	"github.com/nblair2/dingopie/internal/primary"
	"github.com/nblair2/dingopie/internal/secondary"
	"github.com/nblair2/dingopie/internal/shell"
	"github.com/spf13/cobra"
)

var clientCmd = &cobra.Command{
	GroupID: "role",
	Use:     "client <mode> <action>",
	Short:   "run as DNP3 master",
	Long:    internal.Banner + `dingopie client acts as a DNP3 master, using DNP3 Requests Frames.`,
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		if serverIP == "" {
			fmt.Println("Error: server-ip is required")
			os.Exit(1)
		}
		preRun(cmd)
	},
}

var clientDirectCmd = &cobra.Command{
	GroupID: "mode",
	Use:     "direct <action>",
	Short:   "create a new DNP3 channel",
	Long: internal.Banner + `dingopie client direct acts as a DNP3 master, initiating a connection
to the server and sending DNP3 Request Frames.`,
}

var clientDirectSendCmd = &cobra.Command{
	GroupID: "action",
	Use:     "send",
	Short:   "send data to server",
	Run: func(_ *cobra.Command, args []string) {
		if 0 >= points || points > 48 {
			fmt.Println("Error: points cannot be less than 0 or greater than 48")

			return
		}

		if -1 > pointVariance || pointVariance > 1 {
			fmt.Println("Error: point-variance must be between -1 and 1")

			return
		}

		data, err := getData(file, args)
		if err != nil {
			fmt.Printf("Error getting data: %v\n", err)
			os.Exit(1)
		}

		err = primary.ClientSend(serverIP, serverPort, key, data, points, pointVariance, wait)
		if err != nil {
			fmt.Printf(
				"Error with direct send: %v", err)
			os.Exit(1)
		}
	},
}

var clientDirectReceiveCmd = &cobra.Command{
	GroupID: "action",
	Use:     "receive",
	Short:   "receive data from server",
	Run: func(_ *cobra.Command, _ []string) {
		var f *os.File
		var err error
		if file != "" {
			f, err = os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o400)
			if err != nil {
				fmt.Printf("Error opening file %s: %v\n", file, err)
				os.Exit(1)
			}
			defer f.Close()
		}

		data, err := secondary.ClientReceive(serverIP, serverPort, key, wait)
		if err != nil {
			fmt.Printf(
				"Error with direct receive: %v\nAttempting to output what data we have\n",
				err,
			)
		}

		if file != "" {
			_, err := f.Write(data)
			if err != nil {
				fmt.Printf("Error writing to file: %v\n", err)
				fmt.Printf(">> Data received: %s\n", string(data))
				os.Exit(1)
			}
			fmt.Printf(">> Data written to %s\n", file)
		} else {
			fmt.Printf(">> Message: %s\n", string(data))
		}
	},
}

var clientDirectShellCmd = &cobra.Command{
	GroupID: "action",
	Use:     "shell",
	Short:   "run a pty shell on this device",
	Run: func(_ *cobra.Command, _ []string) {
		err := shell.ClientShell(serverIP, serverPort, key, command)
		if err != nil {
			fmt.Printf("Error with direct shell: %v\n", err)
			os.Exit(1)
		}
	},
}

var clientDirectConnectCmd = &cobra.Command{
	GroupID: "action",
	Use:     "connect",
	Short:   "connect to a pty shell running on server",
	Run: func(_ *cobra.Command, _ []string) {
		err := shell.ClientConnect(serverIP, serverPort, key)
		if err != nil {
			fmt.Printf("Error with direct connect: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(">> Connection closed")
	},
}

func init() {
	clientCmd.AddGroup(&cobra.Group{ID: "mode", Title: "Modes:"})
	clientCmd.AddCommand(clientDirectCmd)
	clientDirectCmd.AddGroup(&cobra.Group{ID: "action", Title: "Actions:"})
	clientDirectCmd.AddCommand(clientDirectSendCmd)
	clientDirectCmd.AddCommand(clientDirectReceiveCmd)
	clientDirectCmd.AddCommand(clientDirectShellCmd)
	clientDirectCmd.AddCommand(clientDirectConnectCmd)

	clientDirectCmd.PersistentFlags().
		DurationVarP(&wait, "wait", "w", 1*time.Second, "wait time between DNP3 requests")
	clientDirectSendCmd.PersistentFlags().
		StringVarP(&file, "file", "f", "", "file to read data from (default is a positional argument)")
	clientDirectReceiveCmd.PersistentFlags().
		StringVarP(&file, "file", "f", "", "file to write data to (default is to stdout)")
	clientDirectSendCmd.PersistentFlags().
		IntVarP(&points, "points", "o", 8, "number of 4-byte points to send in each message (max 48)")
	clientDirectSendCmd.PersistentFlags().
		Float32VarP(&pointVariance, "point-variance", "r", 0.25,
			"variance of points to send in each message (e.g., 0.25 = ±25%)")
	clientDirectShellCmd.PersistentFlags().
		StringVarP(&command, "command", "c", os.Getenv("SHELL"), "command to run")
}
