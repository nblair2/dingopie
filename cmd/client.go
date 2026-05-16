package cmd

import (
	"os"
	"time"

	"github.com/nblair2/dingopie/internal"
	"github.com/nblair2/dingopie/internal/inject"
	"github.com/nblair2/dingopie/internal/primary"
	"github.com/nblair2/dingopie/internal/secondary"
	"github.com/nblair2/dingopie/internal/shell"
	"github.com/spf13/cobra"
)

var clientCmd = &cobra.Command{
	GroupID: groupRole,
	Use:     "client <mode> <action>",
	Short:   "run as DNP3 master",
	Long:    internal.Banner + `dingopie client acts as a DNP3 master, using DNP3 Requests Frames.`,
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		if serverIP == "" {
			cmd.Println("Error: server-ip is required")
			os.Exit(1)
		}

		preRun(cmd)
	},
}

var clientDirectCmd = &cobra.Command{
	GroupID: groupMode,
	Use:     "direct <action>",
	Short:   "create a new DNP3 channel",
	Long: internal.Banner + `dingopie client direct acts as a DNP3 master, initiating a connection
to the server and sending DNP3 Request Frames.`,
}

var clientDirectSendCmd = &cobra.Command{
	GroupID: groupAction,
	Use:     useSend,
	Short:   "send data to server",
	Run: func(cmd *cobra.Command, args []string) {
		if 0 >= points || points > 48 {
			cmd.Println("Error: points cannot be less than 0 or greater than 48")

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

		err = primary.ClientSend(
			cmd.OutOrStdout(),
			serverIP,
			serverPort,
			key,
			data,
			points,
			pointVariance,
			wait,
		)
		if err != nil {
			cmd.Printf(
				"Error with direct send: %v", err)
			os.Exit(1)
		}
	},
}

var clientDirectReceiveCmd = &cobra.Command{
	GroupID: groupAction,
	Use:     useRecv,
	Short:   "receive data from server",
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

		data, err := secondary.ClientReceive(cmd.OutOrStdout(), serverIP, serverPort, key, wait)
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
				cmd.Printf(">> Data received: %s\n", string(data))
				os.Exit(1)
			}

			cmd.Printf(">> Data written to %s\n", file)
		} else {
			cmd.Printf(">> Message: %s\n", string(data))
		}
	},
}

var clientDirectShellCmd = &cobra.Command{
	GroupID: groupAction,
	Use:     useShell,
	Short:   "run a pty shell on this device",
	Run: func(cmd *cobra.Command, _ []string) {
		err := shell.ClientShell(cmd.OutOrStdout(), serverIP, serverPort, key, command)
		if err != nil {
			cmd.Printf("Error with direct shell: %v\n", err)
			os.Exit(1)
		}
	},
}

var clientDirectConnectCmd = &cobra.Command{
	GroupID: groupAction,
	Use:     useConnect,
	Short:   "connect to a pty shell running on server",
	Run: func(cmd *cobra.Command, _ []string) {
		err := shell.ClientConnect(cmd.OutOrStdout(), serverIP, serverPort, key)
		if err != nil {
			cmd.Printf("Error with direct connect: %v\n", err)
			os.Exit(1)
		}

		cmd.Println(">> Connection closed")
	},
}

var clientInjectCmd = &cobra.Command{
	GroupID: groupMode,
	Use:     "inject <action>",
	Short:   "inject into an existing DNP3 channel",
	Long: internal.Banner +
		`dingopie client inject runs on an existing DNP3 master, adding data to DNP3 requests and extracting data from` +
		`DNP3 responses.`,
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		preRun(cmd)
	},
}

var clientInjectReceiveCmd = &cobra.Command{
	GroupID: groupAction,
	Use:     useRecv,
	Short:   "receive data from server",
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

		data, err := inject.ClientInjectReceive(
			cmd.OutOrStdout(), clientIP, serverIP, clientPort, serverPort, key,
		)
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

var clientInjectSendCmd = &cobra.Command{
	GroupID: groupAction,
	Use:     useSend,
	Short:   "send data to server",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := getData(cmd, file, args)
		if err != nil {
			cmd.Printf("Error getting data: %v\n", err)
			os.Exit(1)
		}

		err = inject.ClientInjectSend(
			cmd.OutOrStdout(), clientIP, serverIP, clientPort, serverPort, key, data,
		)
		if err != nil {
			cmd.Printf("Error with inject send: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	clientCmd.AddGroup(&cobra.Group{ID: groupMode, Title: titleMode})
	clientCmd.AddCommand(clientDirectCmd)

	clientDirectCmd.AddGroup(&cobra.Group{ID: groupAction, Title: titleAction})
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
		IntVarP(&points, "points", "o", defaultPoints, "number of 4-byte points to send in each message (max 48)")
	clientDirectSendCmd.PersistentFlags().
		Float32VarP(&pointVariance, "point-variance", "r", defaultPointVariance,
			"variance of points to send in each message (e.g., 0.25 = ±25%)")
	clientDirectShellCmd.PersistentFlags().
		StringVarP(&command, "command", "c", os.Getenv("SHELL"), "command to run")

	clientCmd.AddCommand(clientInjectCmd)
	clientInjectCmd.AddGroup(&cobra.Group{ID: groupAction, Title: titleAction})
	clientInjectCmd.PersistentFlags().
		StringVarP(&clientIP, "client-ip", "j", "", "client IP address to filter on (default is all addresses)")
	clientInjectCmd.PersistentFlags().
		IntVarP(&clientPort, "client-port", "q", 0, "client port to filter on (default is all ports)")
	clientInjectCmd.AddCommand(clientInjectReceiveCmd)
	clientInjectReceiveCmd.PersistentFlags().
		StringVarP(&file, "file", "f", "", "file to write data to (default is to stdout)")
	clientInjectCmd.AddCommand(clientInjectSendCmd)
	clientInjectSendCmd.PersistentFlags().
		StringVarP(&file, "file", "f", "", "file to read data from (default is a positional argument)")
}
