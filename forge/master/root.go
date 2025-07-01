package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	srcAddr, dstAddr, key string
	srcPrt, dstPrt        uint16
	banner                = `
 ▌▘        ▘    ▄▖▄▖▄▖▄▖▄▖  ▗▘       ▗     ▝▖
▛▌▌▛▌▛▌▛▌▛▌▌█▌▄▖▙▖▌▌▙▘▌ ▙▖  ▐ ▛▛▌▀▌▛▘▜▘█▌▛▘ ▌
▙▌▌▌▌▙▌▙▌▙▌▌▙▖  ▌ ▙▌▌▌▙▌▙▖  ▐ ▌▌▌█▌▄▌▐▖▙▖▌  ▌
     ▄▌  ▌                  ▝▖             ▗▘`

	rootCmd = &cobra.Command{
		Use:   "dp-forge",
		Short: "dingopie forge mode: creates its own DNP3 packets",
		Long:  banner,
		Run:   func(cmd *cobra.Command, args []string) {},
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.EnableCommandSorting = false

	rootCmd.PersistentFlags().StringVarP(&srcAddr, "src", "s", "",
		"source address\t\t(default is all addresses)")
	rootCmd.PersistentFlags().StringVarP(&dstAddr, "dst", "d", "",
		"destination address\t(default is all addresses)")
	rootCmd.PersistentFlags().Uint16VarP(&srcPrt, "sport", "p", 20000,
		"source port\t\t(use 0 for all ports)")
	rootCmd.PersistentFlags().Uint16VarP(&dstPrt, "dport", "P", 0,
		"destination port\t\t(default is all ports)")
	rootCmd.PersistentFlags().StringVarP(&key, "key", "k", "",
		"encryption key\t\t(default is no encryption)")
	rootCmd.Flags().SortFlags = false
	rootCmd.PersistentFlags().SortFlags = false
}
