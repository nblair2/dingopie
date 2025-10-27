package forge

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	objects int

	serverBanner = `
 ▌▘        ▘    ▄▖▄▖▄▖▄▖▄▖  ▗▘    ▗   ▗   ▗ ▘    ▝▖
▛▌▌▛▌▛▌▛▌▛▌▌█▌▄▖▙▖▌▌▙▘▌ ▙▖  ▐ ▛▌▌▌▜▘▛▘▜▘▀▌▜▘▌▛▌▛▌ ▌
▙▌▌▌▌▙▌▙▌▙▌▌▙▖  ▌ ▙▌▌▌▙▌▙▖  ▐ ▙▌▙▌▐▖▄▌▐▖█▌▐▖▌▙▌▌▌ ▌
     ▄▌  ▌      ~cgwcc~     ▝▖                   ▗▘

`

	ServerCmd = &cobra.Command{
		Use:   "server",
		Short: "dingopie forge mode, server (DNP3 outstation), sends data",
		Long:  serverBanner + "This chicanery brought to you by the Camp George West Computer Club",
		Example: `  dingopie-forge-server -f /path/to/file.txt
  dingopie-forge-server "my secret message inline" -p 20001 -k "password"`,
		Args: func(_ *cobra.Command, args []string) error {
			if file == "" && len(args) == 0 {
				return errors.New("must provide -f file or string positional argument")
			}
			if file != "" && len(args) > 0 {
				return errors.New("cannot use both -f flag and positional argument")
			}

			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			return runServer(args)
		},
	}
)

func init() {
	ServerCmd.Flags().StringVarP(&file, "file", "f", "",
		"file to read data from (default is command line)")
	ServerCmd.Flags().StringVarP(&key, "key", "k", "",
		"encryption key (default is no encryption)")
	ServerCmd.Flags().Uint16VarP(&port, "port", "p", 20000,
		"port to listen on (default is 20000)")
	ServerCmd.Flags().IntVarP(&objects, "objects", "o", 10,
		"number of 32bit objects to send in each response."+
			" Higher for increased bandwidth")
}

func runServer(args []string) error {
	chunk := DNP3ObjSize * objects

	data, err := setupServer(args)
	if err != nil {
		return fmt.Errorf("error setting up: %w", err)
	}

	if key != "" {
		fmt.Println(">> Encrypting data")

		data = xorData(key, data)
	}

	server, err := NewServer(port)
	if err != nil {
		return fmt.Errorf("error creating server: %w", err)
	}

	err = server.RunServer(data, chunk)
	if err != nil {
		return fmt.Errorf("error running server: %w", err)
	}

	err = server.Close()
	if err != nil {
		return fmt.Errorf("error closing server: %w", err)
	}

	fmt.Println("DONE!")

	return nil
}

func setupServer(args []string) ([]byte, error) {
	fmt.Print(serverBanner)
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
