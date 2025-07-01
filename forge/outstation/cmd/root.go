package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"dingopie/forge/outstation/app"
)

var (
	key    string
	port   uint16
	banner = `
 ▌▘        ▘    ▄▖▄▖▄▖▄▖▄▖  ▗▘    ▗   ▗   ▗ ▘    ▝▖
▛▌▌▛▌▛▌▛▌▛▌▌█▌▄▖▙▖▌▌▙▘▌ ▙▖  ▐ ▛▌▌▌▜▘▛▘▜▘▀▌▜▘▌▛▌▛▌ ▌
▙▌▌▌▌▙▌▙▌▙▌▌▙▖  ▌ ▙▌▌▌▙▌▙▖  ▐ ▙▌▙▌▐▖▄▌▐▖█▌▐▖▌▙▌▌▌ ▌
     ▄▌  ▌                  ▝▖                   ▗▘
`

	rootCmd = &cobra.Command{
		Use:   "dingopie-forge-outstation",
		Short: "dingopie forge mode: creates its own DNP3 packets",
		Long:  banner,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf(banner+`
Running dingopie forge mode, as a DNP3 outstation (server)
>> Port: %d
>> Key : %s
>> Waiting for connections...`,
				port, key)

			app.CreateDNP3Packet()

			fmt.Println(`
>> Cleaning up...
DONE!`)
		},
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.EnableCommandSorting = false

	rootCmd.PersistentFlags().Uint16VarP(&port, "port", "p", 20000,
		"port to listen for DNP3 connections on")
	rootCmd.PersistentFlags().StringVarP(&key, "key", "k", "",
		"encryption key (default is no encryption)")
	rootCmd.Flags().SortFlags = false
	rootCmd.PersistentFlags().SortFlags = false

}
