package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	banner = `
 ▌▘        ▘    ▄▖▄▖▄▖▄▖▄▖  ▗▘    ▗   ▗   ▗ ▘    ▝▖
▛▌▌▛▌▛▌▛▌▛▌▌█▌▄▖▙▖▌▌▙▘▌ ▙▖  ▐ ▛▌▌▌▜▘▛▘▜▘▀▌▜▘▌▛▌▛▌ ▌
▙▌▌▌▌▙▌▙▌▙▌▌▙▖  ▌ ▙▌▌▌▙▌▙▖  ▐ ▙▌▙▌▐▖▄▌▐▖█▌▐▖▌▙▌▌▌ ▌
     ▄▌  ▌                  ▝▖                   ▗▘

`
	example = `  dingopie-forge-outstation -f /path/to/file.txt
  dingopie-forge-outstation "my secret message inline" -p 20001 -k "onetimepad"`

	file, key string
	port      uint16

	rootCmd = &cobra.Command{
		Use:     "dingopie-forge-outstation {\"my message\" | file.txt -f}",
		Short:   "dingopie forge mode: creates its own DNP3 packets",
		Long:    banner,
		Example: example,
		Args:    cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {

			var (
				data []byte
				err  error
			)

			fmt.Print(banner)
			fmt.Print("Running dingopie forge mode, as a DNP3 outstation\n")
			fmt.Printf(">> Settings:\n>>>> Port: %d\n", port)
			if key != "" {
				fmt.Printf(">>>> Key : %s", key)
			}

			if file != "" {
				data, err = os.ReadFile(file)
				fmt.Printf(">>>> File: %s\n", file)
				if err != nil {
					fmt.Printf("ERROR: Could not read file %s: %v",
						file, err)
					return
				}
			} else if len(args) > 0 {
				data = []byte(args[0])
			} else {
				fmt.Print("ERROR: Must provide either file with -f" +
					" or string positional\n")
				return
			}

			// Encrypt data

			err = RunServer(port, data)
			if err != nil {
				fmt.Printf("ERROR: server exited with error: %v\n", err)
				return
			}

			fmt.Println("DONE!")
		},
	}
)

func init() {
	cobra.EnableCommandSorting = false

	rootCmd.PersistentFlags().StringVarP(&file, "file", "f", "",
		"file to read data from (default is read from command line)")
	rootCmd.PersistentFlags().Uint16VarP(&port, "port", "p", 20000,
		"port to listen for DNP3 connections on")
	rootCmd.PersistentFlags().StringVarP(&key, "key", "k", "",
		"encryption key (default is no encryption)")
	rootCmd.Flags().SortFlags = false
	rootCmd.PersistentFlags().SortFlags = false

}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
