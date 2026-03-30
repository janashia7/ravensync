package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ravensync",
	Short: "Privacy-first, cross-platform AI memory",
	Long:  "Ravensync is a unified, always-on memory layer for AI agents that works across Telegram, WhatsApp, web, and mobile — with zero knowledge of your data.",
	Run: func(cmd *cobra.Command, args []string) {
		printHelp()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(versionCmd)

	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		printHelp()
	})
}

func printHelp() {
	fmt.Println()
	fmt.Println("  " + cliTitle.Render("Ravensync") + cliSubtle.Render(" — privacy-first AI memory"))
	fmt.Println()
	fmt.Println(cliSubtle.Render("  Encrypted, cross-platform memory for your AI assistant."))
	fmt.Println(cliSubtle.Render("  Your data never leaves your device."))
	fmt.Println()
	fmt.Println(cliText.Render("  Usage:"))
	fmt.Println()
	fmt.Println("    " + cliHighlight.Render("ravensync init") + cliSubtle.Render("       Interactive setup wizard"))
	fmt.Println("    " + cliHighlight.Render("ravensync serve") + cliSubtle.Render("      Start the agent with TUI dashboard"))
	fmt.Println("    " + cliHighlight.Render("ravensync doctor") + cliSubtle.Render("     Check config and dependencies"))
	fmt.Println("    " + cliHighlight.Render("ravensync stats") + cliSubtle.Render("      Show usage statistics"))
	fmt.Println("    " + cliHighlight.Render("ravensync config") + cliSubtle.Render("     Interactive menu (or show / set / allow-users)"))
	fmt.Println("    " + cliHighlight.Render("ravensync version") + cliSubtle.Render("    Print version info"))
	fmt.Println()
	fmt.Println(cliSubtle.Render("  Run any command with --help for more details."))
	fmt.Println()
}
