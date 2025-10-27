package forge

import (
	"github.com/spf13/cobra"
)

var (
	file     string
	key      string
	port     uint16
	ForgeCmd = &cobra.Command{
		Use:   "forge",
		Short: "dingopie forge mode: creates its own DNP3 packets",
		Long: `
 ▌▘        ▘    ▄▖▄▖▄▖▄▖▄▖
▛▌▌▛▌▛▌▛▌▛▌▌█▌▄▖▙▖▌▌▙▘▌ ▙▖
▙▌▌▌▌▙▌▙▌▙▌▌▙▖  ▌ ▙▌▌▌▙▌▙▖
     ▄▌  ▌      ~cgwcc~   
This chicanery brought to you by the Camp George West Computer Club
`,
		Example: `  dingopie forge server
  dingopie forge client`,
	}
)

func init() {
	ForgeCmd.AddCommand(ClientCmd)
	ForgeCmd.AddCommand(ServerCmd)
}
