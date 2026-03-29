package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ravensync/ravensync/internal/config"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check Ravensync configuration and dependencies",
	RunE:  runDoctor,
}

func runDoctor(cmd *cobra.Command, args []string) error {
	printBanner("doctor")

	allGood := true

	cfg, err := config.Load()
	if err != nil {
		printFail("Config: %v", err)
		printNote("Run 'ravensync init' to create configuration.")
		return nil
	}
	printOK("Config loaded from %s", config.ConfigPath(cfg))

	if info, err := os.Stat(cfg.DataDir); err != nil || !info.IsDir() {
		printFail("Data directory missing: %s", cfg.DataDir)
		allGood = false
	} else {
		printOK("Data directory: %s", cfg.DataDir)
	}

	if len(cfg.EncryptionSalt) == 0 {
		printFail("Encryption salt not configured")
		allGood = false
	} else {
		printOK("Encryption salt configured (%d bytes)", len(cfg.EncryptionSalt))
	}

	if cfg.TelegramToken == "" {
		if os.Getenv("RAVENSYNC_TELEGRAM_TOKEN") != "" {
			printOK("Telegram token (from env)")
		} else {
			printWarn("Telegram token not configured")
			allGood = false
		}
	} else {
		printOK("Telegram token configured")
	}

	if cfg.LLMAPIKey == "" {
		if cfg.LLMProvider == "ollama" {
			printOK("LLM: %s / %s (local)", cfg.LLMProvider, cfg.LLMModel)
		} else if os.Getenv("RAVENSYNC_LLM_KEY") != "" {
			printOK("LLM API key (from env)")
		} else {
			printWarn("LLM API key not configured")
			allGood = false
		}
	} else {
		printOK("LLM: %s / %s", cfg.LLMProvider, cfg.LLMModel)
	}

	printOK("Embedding model: %s", cfg.EmbeddingModel)

	dbPath := filepath.Join(cfg.DataDir, "memory.db")
	if info, err := os.Stat(dbPath); err == nil {
		printOK("Memory database: %s (%s)", dbPath, humanSize(info.Size()))
	} else {
		printNote("Memory database will be created on first run")
	}

	fmt.Println()
	if allGood {
		printLine("All checks passed. Run %s to start.", cliHighlight.Render("ravensync serve"))
	} else {
		printWarn("Some checks need attention. See warnings above.")
	}
	fmt.Println()
	return nil
}

func humanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
