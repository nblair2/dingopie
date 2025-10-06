package main

import (
	"errors"
	"fmt"
	"os"

	"dingopie/common"
	"github.com/spf13/cobra"
)

var (
	banner = `
 ▌▘        ▘    ▄▖▄▖▄▖▄▖▄▖  ▗▘    ▗   ▗   ▗ ▘    ▝▖
▛▌▌▛▌▛▌▛▌▛▌▌█▌▄▖▙▖▌▌▙▘▌ ▙▖  ▐ ▛▌▌▌▜▘▛▘▜▘▀▌▜▘▌▛▌▛▌ ▌
▙▌▌▌▌▙▌▙▌▙▌▌▙▖  ▌ ▙▌▌▌▙▌▙▖  ▐ ▙▌▙▌▐▖▄▌▐▖█▌▐▖▌▙▌▌▌ ▌
     ▄▌  ▌      ~cgwcc~     ▝▖                   ▗▘

`
	long    = "This chicanery brought to you by the Camp George West Computer Club"
	example = `  dingopie-forge-server -f /path/to/file.txt
  dingopie-forge-server "my secret message inline" -p 20001 -k "password"`

	file, key string
	port      uint16
	objects   int

	rootCmd = &cobra.Command{
		Use:     "dingopie-forge-server {\"my message\" | -f file.txt}",
		Short:   "dingopie forge mode: creates its own DNP3 packets",
		Long:    banner + long,
		Example: example,
		Args: func(cmd *cobra.Command, args []string) error {
			if file == "" && len(args) == 0 {
				return errors.New("must provide -f file or string positional argument")
			}
			if file != "" && len(args) > 0 {
				return errors.New("cannot use both -f flag and positional argument")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoot(args)
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
	rootCmd.PersistentFlags().IntVarP(&objects, "objects", "o", 10,
		"number of 32bit objects to send in each response."+
			" Higher for increased bandwidth")
	rootCmd.Flags().SortFlags = false
	rootCmd.PersistentFlags().SortFlags = false
}

func runRoot(args []string) error {
	chunk := common.DNP3_OBJ_SIZE * objects

	data, err := setup(args)
	if err != nil {
		return fmt.Errorf("error setting up: %w", err)
	}

	if key != "" {
		fmt.Println(">> Encrypting data")

		data = common.XORData(key, data)
	}

	s, err := NewServer(port)
	if err != nil {
		return fmt.Errorf("error creating server: %w", err)
	}

	err = s.RunServer(data, chunk)
	if err != nil {
		return fmt.Errorf("error running server: %w", err)
	}

	err = s.Close()
	if err != nil {
		return fmt.Errorf("error closing server: %w", err)
	}

	fmt.Println("DONE!")

	return nil
}

func setup(args []string) ([]byte, error) {
	fmt.Print(banner)
	fmt.Print("Running dingopie forge mode, as a DNP3 outstation\n")
	fmt.Printf(">> Settings:\n>>>> Port   : %d\n", port)
	fmt.Printf(">>>> Objects: %d (x4 bytes each)\n", objects)

	if key != "" {
		fmt.Printf(">>>> Key    : %s\n", key)
	}

	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("could not read file %s: %w", file, err)
		}

		fmt.Printf(">>>> File   : %s\n", file)

		return data, nil
	} else if len(args) > 0 {
		return []byte(args[0]), nil
	}

	return nil, errors.New("must provide -f file or string positional")
}
