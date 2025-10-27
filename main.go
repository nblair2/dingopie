package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"dingopie/forge"
)

var rootCmd = &cobra.Command{
	Use:   "dingopie forge {server|client}",
	Short: "dingopie, a DNP3 covert channel",
	Long: `
	   |\_/|     ▌▘        ▘       ) (
	  /     \   ▛▌▌▛▌▛▌▛▌▛▌▌█▌    ) ( )
	 /_ ~ ~ _\  ▙▌▌▌▌▙▌▙▌▙▌▌▙▖  .:::::::.
	    \@/          ▄▌  ▌     ~\_______/~

This chicanery brought to you by the Camp George West Computer Club
`,
	Example: `  dingopie forge`,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
}

func init() {
	rootCmd.AddCommand(forge.ForgeCmd)
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
