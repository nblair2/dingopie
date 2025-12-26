// Package cmd implements dingopie cli with cobra
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const banner = `
▓█████▄  ██▓ ███▄    █   ▄████  ▒█████   ██▓███   ██▓▓█████ 
▒██▀ ██▌▓██▒ ██ ▀█   █  ██▒ ▀█▒▒██▒  ██▒▓██░  ██▒▓██▒▓█   ▀ 
░██   █▌▒██▒▓██  ▀█ ██▒▒██░▄▄▄░▒██░  ██▒▓██░ ██▓▒▒██▒▒███   
░▓█▄   ▌░██░▓██▒  ▐▌██▒░▓█  ██▓▒██   ██░▒██▄█▓▒ ▒░██░▒▓█  ▄ 
░▒████▓ ░██░▒██░   ▓██░░▒▓███▀▒░ ████▓▒░▒██▒ ░  ░░██░░▒████▒
 ▒▒▓  ▒ ░▓  ░ ▒░   ▒ ▒  ░▒   ▒ ░ ▒░▒░▒░ ▒▓▒░ ░  ░░▓  ░░ ▒░ ░
 ░ ▒  ▒  ▒ ░░ ░░   ░ ▒░  ░   ░   ░ ▒ ▒░ ░▒ ░      ▒ ░ ░ ░  ░

      |\__/|     This skullduggery brought       ) (
     /     \     to you by the Camp George      ) ( )
    /_.~ ~,_\        West Computer Club       :::::::::
       \@/                                   ~\_______/~
`

func printBanner(cmd *cobra.Command, args []string) {
	fmt.Println(
		strings.ReplaceAll(fmt.Sprintf("======== %s ========", cmd.CommandPath()), " ", " | "),
	)
}

var rootCmd = &cobra.Command{
	Use:   "dingopie",
	Short: "dingopie is a DNP3 covert channel",
	Long: banner + `
dingopie is a DNP3 covert channel.
It supports server and client roles, direct and inject modes, and various actions.`,
	PersistentPreRun:  printBanner,
	PersistentPostRun: printBanner,
}

// Execute entrypoint for dingopie CLI.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringP("key", "k", "", "encryption key (default is no encryption)")
	rootCmd.PersistentFlags().String("server-ip", "", "Server IP address")
	rootCmd.PersistentFlags().Int("server-port", 20000, "Server port")
	rootCmd.PersistentFlags().String("client-ip", "", "Client IP address")
	rootCmd.PersistentFlags().Int("client-port", 0, "Client port")
}
