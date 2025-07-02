package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	key    string
	port   uint16
	file   bool
	banner = `
 ▌▘        ▘    ▄▖▄▖▄▖▄▖▄▖  ▗▘    ▗   ▗   ▗ ▘    ▝▖
▛▌▌▛▌▛▌▛▌▛▌▌█▌▄▖▙▖▌▌▙▘▌ ▙▖  ▐ ▛▌▌▌▜▘▛▘▜▘▀▌▜▘▌▛▌▛▌ ▌
▙▌▌▌▌▙▌▙▌▙▌▌▙▖  ▌ ▙▌▌▌▙▌▙▖  ▐ ▙▌▙▌▐▖▄▌▐▖█▌▐▖▌▙▌▌▌ ▌
     ▄▌  ▌                  ▝▖                   ▗▘
`

	rootCmd = &cobra.Command{
		Use:   "dingopie-forge-outstation {-file|-string}",
		Short: "dingopie forge mode: creates its own DNP3 packets",
		Long:  banner,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf(banner)
			fmt.Printf("Running dingopie forge mode, as a DNP3 outstation (server)")
			fmt.Printf(">> Settings:\n>>>> Port: %d\n>>>> Key : %s", port, key)

			var (
				data []byte
				err  error
			)
			if file {
				data, err = os.ReadFile(os.Args[0])
				if err != nil {
					fmt.Println("ERROR: Could not read file %s: %v",
						os.Args[0], err)
					return
				}
			} else {
				data = []byte(os.Args[0])
			}

			RunServer(port, key, data)

			fmt.Println("DONE!")
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

	rootCmd.PersistentFlags().BoolVarP(&file, "-file", "-f", false,
		"read data from a file (default is false, read from command line)")
	rootCmd.PersistentFlags().Uint16VarP(&port, "port", "p", 20000,
		"port to listen for DNP3 connections on")
	rootCmd.PersistentFlags().StringVarP(&key, "key", "k", "",
		"encryption key (default is no encryption)")
	rootCmd.Flags().SortFlags = false
	rootCmd.PersistentFlags().SortFlags = false

}

func main() {
	Execute()
}
