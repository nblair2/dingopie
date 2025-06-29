package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"dingopie/outstation"
)

var (
	rootCmd = &cobra.Command{
		Use:   "dingopie",
		Short: "dingopie - a DNP3 covert channel",
		Long: `
        _ _                         _      
       | (_)                       (_)     
     __| |_ _ __   __ _  ___  _ __  _  ___ 
    / _  | | '_ \ / _  |/ _ \| '_ \| |/ _ \
   | (_| | | | | | (_| | (_) | |_) | |  __/
    \__,_|_|_| |_|\__, |\___/| .__/|_|\___|
                    _/ |     | |           
                   |___/     |_|           

              |\_/|           ) (
             /     \         ) ( )
            /_ ~ ~ _\      .:::::::.
               \@/        ~\_______/~

		`,
		Run: func(cmd *cobra.Command, args []string) {},
	}
	srcAddr, dstAddr, key string
	srcPrt, dstPrt        uint16
)

func buildFWRule(sa, da string, sp, dp uint16) []string {
	var rule = []string{"--protocol", "tcp"}

	if sa != "" {
		rule = append(rule, "--source", sa)
	}
	if da != "" {
		rule = append(rule, "--destination", da)
	}
	if sp != 0 {
		rule = append(rule, "--source-port", fmt.Sprintf("%d", sp))
	}
	if dp != 0 {
		rule = append(rule, "--destination-port", fmt.Sprintf("%d", dp))
	}

	return rule
}

func newOutstationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "outstation",
		Aliases: []string{"o", "out", "slave", "s", "server", "serve"},
		Short:   "run dingopie on a DNP3 outstation",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	return cmd
}

func newOutstationReceiveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "receive",
		Aliases: []string{"r", "receive"},
		Short:   "receive information",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	return cmd
}

func newOutstationSendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "send",
		Aliases: []string{"s", "snd"},
		Short:   "send information",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	return cmd
}

func newOutstationSendFileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "file",
		Aliases: []string{"f"},
		Short:   "send a file",
		Example: "dingopie outstation send file /path/to/file.txt",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {

			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("error reading file %s: %w", args[1], err)
			}

			rule := buildFWRule(srcAddr, dstAddr, srcPrt, dstPrt)
			err = outstation.Send(data, rule)
			if err != nil {
				return fmt.Errorf("error sending data: %w", err)
			}

			return nil
		},
	}
	return cmd
}

func newOutstationSendStrCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "string",
		Aliases: []string{"s", "str"},
		Short:   "send a string (read in from the command line)",
		Example: "dingopie outstation send string \"some secret message\"",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {

			rule := buildFWRule(srcAddr, dstAddr, srcPrt, dstPrt)
			err := outstation.Send([]byte(args[0]), rule)
			if err != nil {
				return fmt.Errorf("error sending data: %w", err)
			}

			return nil
		},
	}
	return cmd
}

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

	outstationSendCmd := newOutstationSendCmd()
	outstationSendCmd.AddCommand(newOutstationSendFileCmd())
	outstationSendCmd.AddCommand(newOutstationSendStrCmd())

	outstationReceiveCmd := newOutstationReceiveCmd()

	outstationCmd := newOutstationCmd()
	outstationCmd.AddCommand(outstationSendCmd)
	outstationCmd.AddCommand(outstationReceiveCmd)

	rootCmd.AddCommand(outstationCmd)
}

func initConfig() {

}
