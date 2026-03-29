package cli

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var (
	crimson = lipgloss.Color("160")
	darkRed = lipgloss.Color("88")
	ember   = lipgloss.Color("131")
	ash     = lipgloss.Color("240")
	bone    = lipgloss.Color("252")
	scarlet = lipgloss.Color("196")

	cliTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(crimson)

	cliSubtle = lipgloss.NewStyle().
			Foreground(ash)

	cliText = lipgloss.NewStyle().
		Foreground(bone)

	cliOK = lipgloss.NewStyle().
		Foreground(ember)

	cliFail = lipgloss.NewStyle().
		Foreground(scarlet).
		Bold(true)

	cliWarn = lipgloss.NewStyle().
		Foreground(lipgloss.Color("208"))

	cliInfo = lipgloss.NewStyle().
		Foreground(ash)

	cliHighlight = lipgloss.NewStyle().
			Foreground(crimson).
			Bold(true)
)

func ravenTheme() *huh.Theme {
	t := huh.ThemeDracula()

	t.Focused.Title = t.Focused.Title.Foreground(crimson).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(ash)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(crimson).Bold(true)
	t.Focused.FocusedButton = t.Focused.FocusedButton.Background(crimson).Foreground(lipgloss.Color("0"))
	t.Focused.BlurredButton = t.Focused.BlurredButton.Background(darkRed).Foreground(bone)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(crimson)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(crimson)
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(ash)
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(crimson)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(ember)

	t.Blurred.Title = t.Blurred.Title.Foreground(ash)
	t.Blurred.Description = t.Blurred.Description.Foreground(lipgloss.Color("236"))
	t.Blurred.TextInput.Prompt = t.Blurred.TextInput.Prompt.Foreground(ash)

	return t
}

func printBanner(subtitle string) {
	fmt.Println()
	fmt.Println(cliTitle.Render("  Ravensync") + cliSubtle.Render(" — "+subtitle))
	fmt.Println(cliSubtle.Render("  " + repeat("─", 40)))
	fmt.Println()
}

func printOK(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println("  " + cliOK.Render("✓") + " " + cliText.Render(msg))
}

func printFail(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println("  " + cliFail.Render("✗") + " " + cliText.Render(msg))
}

func printWarn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println("  " + cliWarn.Render("!") + " " + cliText.Render(msg))
}

func printNote(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println("  " + cliInfo.Render("·") + " " + cliSubtle.Render(msg))
}

func printLine(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println("  " + cliText.Render(msg))
}

func repeat(s string, n int) string {
	out := ""
	for range n {
		out += s
	}
	return out
}
