package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	Version = "0.1.0-dev"
	Commit  = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println()
		fmt.Println("  " + cliTitle.Render("Ravensync") + " " + cliText.Render(Version) + cliSubtle.Render(" ("+Commit+")"))
		fmt.Println()
	},
}
