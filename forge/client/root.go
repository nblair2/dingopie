package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"

	"dingopie/forge/common"
)

var (
	banner = `
 ▌▘        ▘    ▄▖▄▖▄▖▄▖▄▖  ▗▘       ▗     ▝▖
▛▌▌▛▌▛▌▛▌▛▌▌█▌▄▖▙▖▌▌▙▘▌ ▙▖  ▐ ▛▛▌▀▌▛▘▜▘█▌▛▘ ▌
▙▌▌▌▌▙▌▙▌▙▌▌▙▖  ▌ ▙▌▌▌▙▌▙▖  ▐ ▌▌▌█▌▄▌▐▖▙▖▌  ▌
     ▄▌  ▌      ~cgwcc~     ▝▖             ▗▘

`
	long    = "This chicanery brought to you by the Camp George West Computer Club"
	example = `  dingopie-forge-client 1.2.3.4
  dingopie-forge-client 1.2.3.4 -p 20001 -f out.txt -k "password"`
	key, file string
	port      uint16
	wait      float32

	rootCmd = &cobra.Command{
		Use:     "dingopie-forge-client <server ip address>",
		Short:   "dingopie forge mode: creates its own DNP3 packets",
		Long:    banner + long,
		Example: example,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoot(args)
		},
	}
)

func init() {
	cobra.EnableCommandSorting = false

	rootCmd.Flags().SortFlags = false
	rootCmd.PersistentFlags().Uint16VarP(&port, "port", "p", 20000,
		"port to connect to DNP3 server")
	rootCmd.PersistentFlags().StringVarP(&key, "key", "k", "",
		"encryption key (default is no encryption)")
	rootCmd.PersistentFlags().Float32VarP(&wait, "wait", "w", 5.0,
		"wait in seconds between polls to the server,"+
			" lower for increased bandwidth")
	rootCmd.PersistentFlags().StringVarP(&file, "file", "f", "",
		"file to write data to (default is write to command line)")
	rootCmd.PersistentFlags().SortFlags = false
}

func runRoot(args []string) error {
	var data []byte
	addr := args[0]

	err := setup(addr)
	if err != nil {
		return fmt.Errorf("error setting up: %w", err)
	}

	client, err := NewClient(addr, port)
	if err != nil {
		return fmt.Errorf("failed to connect to server %s:%d: %v",
			addr, port, err)
	}

	// First ask for the data len
	sizeData, err := client.GetData(common.REQ_SIZE)
	if err != nil {
		return fmt.Errorf("failed getting data length from server: %v", err)
	}
	//bad integer stuff
	size := int(binary.LittleEndian.Uint64(sizeData))

	bar := progressbar.NewOptions(size,
		progressbar.OptionSetDescription(">> Receiving data:"),
		progressbar.OptionSetTheme(progressbar.ThemeASCII),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("bits"),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionShowElapsedTimeOnFinish(),
	)

	for len(data) < size {

		time.Sleep(time.Duration(wait) * time.Second)

		newData, err := client.GetData(common.REQ_DATA)
		if err != nil {
			fmt.Printf(">> failed getting next data: %v (continuing)\n", err)
		} else {
			data = append(data, newData...)
			bar.Add(len(newData))
		}
	}
	client.Close()
	bar.Finish()

	// Remove padding
	if len(data) > size {
		data = data[:size]
	}
	if key != "" {
		fmt.Println("\n>> Decrypting data")
		data = common.XORData(key, data)
	}

	err = writeOut(data)
	if err != nil {
		return fmt.Errorf("failed to write out message: %v", err)
	}
	fmt.Println("DONE!")
	return nil
}

func setup(addr string) error {

	fmt.Print(banner)
	fmt.Print("Running dingopie forge mode, as a DNP3 client\n")
	fmt.Printf(">> Settings:\n>>>> Addr  : %s\n", addr)
	fmt.Printf(">>>> Port  : %d\n>>>> Wait  : %f seconds/request\n", port, wait)

	if key != "" {
		fmt.Printf(">>>> Key : %s\n", key)
	}

	if file != "" {
		_, err := os.Stat(file)
		if err == nil {
			return fmt.Errorf("file %s already exists", file)
		} else if errors.Is(err, os.ErrNotExist) {
			fmt.Printf(">>>> File  : %s\n", file)
		} else {
			return fmt.Errorf("error checking file %s: %v", file, err)
		}
	} else {
		fmt.Print(">>>> Output: stdio\n")
	}

	return nil
}

func writeOut(data []byte) error {
	if file != "" {
		writer, err := os.Create(file)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %v", file, err)
		}
		defer writer.Close()

		_, err = writer.Write(data)
		if err != nil {
			return fmt.Errorf("failed to write data to file %s: %v", file, err)
		}
	} else {
		fmt.Println(">> Message:\n" + string(data))
	}
	return nil
}
