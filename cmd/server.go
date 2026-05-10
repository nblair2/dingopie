package cmd

import (
	"os"

	"github.com/nblair2/dingopie/internal"
	"github.com/nblair2/dingopie/internal/inject"
	"github.com/nblair2/dingopie/internal/primary"
	"github.com/nblair2/dingopie/internal/secondary"
	"github.com/nblair2/dingopie/internal/shell"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	GroupID: groupRole,
	Use:     "server <mode> <action>",
	Short:   "run as DNP3 outstation",
	Long:    internal.Banner + `dingopie server acts as a DNP3 outstation, using DNP3 Response Frames.`,
}

var serverDirectCmd = &cobra.Command{
	GroupID: groupMode,
	Use:     "direct <action>",
	Short:   "create a new DNP3 channel",
	Long: internal.Banner + `dingopie server direct acts as a DNP3 outstation, accepting connections
from the client and sending DNP3 Response Frames.`,
}

var serverDirectSendCmd = &cobra.Command{
	GroupID: groupAction,
	Use:     useSend,
	Short:   "send data to client",
	Run: func(cmd *cobra.Command, args []string) {
		if 0 >= points || points > 60 {
			cmd.Println("Error: points cannot be less than 0 or greater than 60")

			return
		}

		if -1 > pointVariance || pointVariance > 1 {
			cmd.Println("Error: point-variance must be between -1 and 1")

			return
		}

		data, err := getData(cmd, file, args)
		if err != nil {
			cmd.Printf("Error getting data: %v\n", err)
			os.Exit(1)
		}

		err = secondary.ServerSend(
			cmd.OutOrStdout(),
			serverIP,
			serverPort,
			key,
			data,
			points,
			pointVariance,
		)
		if err != nil {
			cmd.Printf("Error with direct send: %v\n", err)
			os.Exit(1)
		}
	},
}

var serverDirectReceiveCmd = &cobra.Command{
	GroupID: groupAction,
	Use:     useRecv,
	Short:   "receive data from client",
	Run: func(cmd *cobra.Command, _ []string) {
		var (
			f   *os.File
			err error
		)

		if file != "" {
			f, err = os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, receiveFileMode)
			if err != nil {
				cmd.Printf("Error opening file %s: %v\n", file, err)
				os.Exit(1)
			}
			defer f.Close()
		}

		data, err := primary.ServerReceive(cmd.OutOrStdout(), serverIP, serverPort, key)
		if err != nil {
			cmd.Printf(
				"Error with direct receive: %v\nAttempting to output what data we have\n",
				err,
			)
		}

		if file != "" {
			_, err := f.Write(data)
			if err != nil {
				cmd.Printf("Error writing to file: %v\n", err)
				cmd.Printf(">> Attempting to output what data we have: %s\n", string(data))
				os.Exit(1)
			}

			cmd.Printf(">> Data written to %s\n", file)
		} else {
			cmd.Printf(">> Message: %s\n", string(data))
		}
	},
}

var serverDirectShellCmd = &cobra.Command{
	GroupID: groupAction,
	Use:     useShell,
	Short:   "run a pty shell on this device",
	Run: func(cmd *cobra.Command, _ []string) {
		err := shell.ServerShell(cmd.OutOrStdout(), serverIP, serverPort, key, command)
		if err != nil {
			cmd.Printf("Error opening shell: %v\n", err)
			os.Exit(1)
		}
	},
}

var serverDirectConnectCmd = &cobra.Command{
	GroupID: groupAction,
	Use:     useConnect,
	Short:   "connect to a pty shell running on client",
	Run: func(cmd *cobra.Command, _ []string) {
		err := shell.ServerConnect(cmd.OutOrStdout(), serverIP, serverPort, key)
		if err != nil {
			cmd.Printf("Error connecting to shell: %v\n", err)
			os.Exit(1)
		}

		cmd.Println(">> Connection closed")
	},
}

var serverInjectCmd = &cobra.Command{
	GroupID: groupMode,
	Use:     "inject <action>",
	Short:   "inject into an existing DNP3 channel",
	Long: internal.Banner +
		`dingopie server inject runs on an existing DNP3 master, adding data to DNP3 responses and extracting data from` +
		` DNP3 requests.`,
}

var serverInjectSendCmd = &cobra.Command{
	GroupID: groupAction,
	Use:     useSend,
	Short:   "send data to client",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := getData(cmd, file, args)
		if err != nil {
			cmd.Printf("Error getting data: %v\n", err)
			os.Exit(1)
		}

		err = inject.ServerInjectSend(serverIP, clientIP, serverPort, clientPort, key, data)
		if err != nil {
			cmd.Printf("Error with inject send: %v\n", err)
			os.Exit(1)
		}
	},
}

//nolint:dupl // temporary mock during building
var serverInjectReceiveCmd = &cobra.Command{
	GroupID: groupAction,
	Use:     useRecv,
	Short:   "receive data from client",
	Run: func(cmd *cobra.Command, _ []string) {
		var (
			f   *os.File
			err error
		)

		if file != "" {
			f, err = os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, receiveFileMode)
			if err != nil {
				cmd.Printf("Error opening file %s: %v\n", file, err)
				os.Exit(1)
			}
			defer f.Close()
		}

		data, err := inject.ServerInjectReceive(serverIP, clientIP, serverPort, clientPort, key)
		if err != nil {
			cmd.Printf(
				"Error with inject receive: %v\nAttempting to output what data we have\n",
				err,
			)
		}

		if file != "" {
			_, err := f.Write(data)
			if err != nil {
				cmd.Printf("Error writing to file: %v\n", err)
				cmd.Printf(">> Data received: %s\n", string(data))
				os.Exit(1)
			}

			cmd.Printf(">> Data written to %s\n", file)
		} else {
			cmd.Printf(">> Message: %s\n", string(data))
		}
	},
}

func init() {
	serverCmd.AddGroup(&cobra.Group{ID: groupMode, Title: titleMode})
	serverCmd.AddCommand(serverDirectCmd)

	serverDirectCmd.AddGroup(&cobra.Group{ID: groupAction, Title: titleAction})
	serverDirectCmd.AddCommand(serverDirectSendCmd)
	serverDirectCmd.AddCommand(serverDirectReceiveCmd)
	serverDirectCmd.AddCommand(serverDirectShellCmd)
	serverDirectCmd.AddCommand(serverDirectConnectCmd)
	serverDirectCmd.AddCommand(serverDirectConnectCmd)
	serverDirectSendCmd.PersistentFlags().
		StringVarP(&file, "file", "f", "", "file to read data from (default is command line)")
	serverDirectReceiveCmd.PersistentFlags().
		StringVarP(&file, "file", "f", "", "file to write data to (default is to stdout)")
	serverDirectSendCmd.PersistentFlags().
		IntVarP(&points, "points", "o", defaultPoints, "number of 4-byte points to send in each message (max 60)")
	serverDirectSendCmd.PersistentFlags().
		Float32VarP(&pointVariance, "point-variance", "r", defaultPointVariance,
			"variance of points to send in each message (e.g., 0.25 = ±25%)")
	serverDirectShellCmd.PersistentFlags().
		StringVarP(&command, "command", "c", os.Getenv("SHELL"), "command to run")

	serverCmd.AddCommand(serverInjectCmd)
	serverInjectCmd.AddGroup(&cobra.Group{ID: groupAction, Title: titleAction})
	serverInjectCmd.PersistentFlags().
		StringVarP(&clientIP, "client-ip", "j", "", "client IP address to filter on (default is all addresses)")
	serverInjectCmd.PersistentFlags().
		IntVarP(&clientPort, "client-port", "q", 0, "client port to filter on (default is all ports)")
	serverInjectCmd.AddCommand(serverInjectSendCmd)
	serverInjectSendCmd.PersistentFlags().
		StringVarP(&file, "file", "f", "", "file to read data from (default is a positional argument)")
	serverInjectCmd.AddCommand(serverInjectReceiveCmd)
	serverInjectReceiveCmd.PersistentFlags().
		StringVarP(&file, "file", "f", "", "file to write data to (default is to stdout)")
}
