package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var (
	banner = `
 ▌▘        ▘    ▄▖▄▖▄▖▄▖▄▖  ▗▘       ▗     ▝▖
▛▌▌▛▌▛▌▛▌▛▌▌█▌▄▖▙▖▌▌▙▘▌ ▙▖  ▐ ▛▛▌▀▌▛▘▜▘█▌▛▘ ▌
▙▌▌▌▌▙▌▙▌▙▌▌▙▖  ▌ ▙▌▌▌▙▌▙▖  ▐ ▌▌▌█▌▄▌▐▖▙▖▌  ▌
     ▄▌  ▌      ~cgwcc~     ▝▖             ▗▘

`
	long    = "This chicanery brought to you by the Camp George West Computer Club"
	example = `  dingopie-forge-master 1.2.3.4
  dingopie-forge-master 1.2.3.4 -p 20001 -f out.txt -k "password"`

	key, file string
	port      uint16
	wait      float32

	rootCmd = &cobra.Command{
		Use:     "dingopie-forge-master <outstation ip address>",
		Short:   "dingopie forge mode: creates its own DNP3 packets",
		Long:    banner + long,
		Example: example,
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var writer io.Writer

			fmt.Print(banner)
			fmt.Print("Running dingopie forge mode, as a DNP3 master\n")
			fmt.Printf(">>Settings:\n>>>> Addr: %s\n", args[0])
			fmt.Printf(">>>> Port: %d\n>>>> Wait: %f seconds\n", port, wait)
			if key != "" {
				fmt.Printf(">>>> Key : %s", key)
			}

			if file != "" {
				fmt.Printf(">>>> File: %s\n", file)
				f, err := os.Create(file)
				if err != nil {
					fmt.Printf("ERROR: Failed to create file %s, %v\n",
						file, err)
					return
				}
				defer f.Close()
				writer = f
			} else {
				fmt.Print(">>>> Output to stdio\n")
				writer = os.Stdout
			}

			data, err := RunClient(args[0], port, wait)
			if err != nil {
				fmt.Printf("ERROR: Running client: %v", err)
				return
			}

			// Decrypt data from password

			if writer == os.Stdout {
				fmt.Print("Message: ")
			}

			n, err := writer.Write(data)
			if err != nil {
				fmt.Printf("ERROR: Failed to write %d bytes to writer %s: %v",
					n, writer, err)
				return
			}

			if writer == os.Stdout {
				fmt.Print("\n")
			}

			fmt.Println("DONE!")
		},
	}
)

func init() {
	cobra.EnableCommandSorting = false

	rootCmd.Flags().SortFlags = false
	rootCmd.PersistentFlags().Uint16VarP(&port, "port", "p", 20000,
		"port to connect to DNP3 outstation")
	rootCmd.PersistentFlags().StringVarP(&key, "key", "k", "",
		"encryption key (default is no encryption)")
	rootCmd.PersistentFlags().Float32VarP(&wait, "wait", "w", 5.0,
		"wait in seconds between polls to the outstation,"+
			" lower for increased bandwidth")
	rootCmd.PersistentFlags().StringVarP(&file, "file", "f", "",
		"file to write data to (default is write to command line)")
	rootCmd.PersistentFlags().SortFlags = false
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
