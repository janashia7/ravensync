package cli

import (
	"crypto/rand"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/spf13/cobra"

	"github.com/ravensync/ravensync/internal/config"
	"github.com/ravensync/ravensync/internal/crypto"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Ravensync configuration",
	Long:  "Interactive setup wizard that creates the Ravensync data directory, config file, and encryption key.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().String("telegram-token", "", "Telegram bot token (skip prompt)")
	initCmd.Flags().String("llm-key", "", "LLM API key (skip prompt)")
	initCmd.Flags().String("llm-provider", "", "LLM provider (skip prompt)")
	initCmd.Flags().String("llm-model", "", "Chat model name (skip prompt)")
}

func runInit(cmd *cobra.Command, args []string) error {
	cfg := config.DefaultConfig()

	if _, err := os.Stat(config.ConfigPath(cfg)); err == nil {
		var overwrite bool
		err := huh.NewConfirm().
			Title("Config already exists").
			Description(fmt.Sprintf("Found existing config at %s", config.ConfigPath(cfg))).
			Affirmative("Overwrite").
			Negative("Cancel").
			Value(&overwrite).
			WithTheme(ravenTheme()).
			Run()
		if err != nil || !overwrite {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	var (
		password       string
		confirmPw      string
		telegramTok    string
		llmProvider    = "openai"
		llmKey         string
		llmModel       = "gpt-4o-mini"
		embeddingModel string
	)

	applyInitFlags(cmd, &telegramTok, &llmProvider, &llmKey, &llmModel)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Ravensync Setup").
				Description("Your data is encrypted locally and never leaves your device.\nLet's get you set up."),

			huh.NewInput().
				Title("Encryption Password").
				Description("Minimum 8 characters. This is never stored anywhere.").
				EchoMode(huh.EchoModePassword).
				Validate(func(s string) error {
					if len(s) < 8 {
						return fmt.Errorf("must be at least 8 characters")
					}
					return nil
				}).
				Value(&password),

			huh.NewInput().
				Title("Confirm Password").
				EchoMode(huh.EchoModePassword).
				Validate(func(s string) error {
					if s != password {
						return fmt.Errorf("passwords do not match")
					}
					return nil
				}).
				Value(&confirmPw),
		).Title("Encryption"),

		huh.NewGroup(
			huh.NewInput().
				Title("Telegram Bot Token").
				Description("Create a bot via @BotFather on Telegram.\nLeave empty to configure later.").
				Placeholder("paste token here or press Enter to skip").
				Value(&telegramTok),
		).Title("Telegram"),

		huh.NewGroup(
			huh.NewSelect[string]().
				Title("LLM Provider").
				Description("Which AI provider do you want to use?").
				Options(
					huh.NewOption("Ollama (local, free)", "ollama"),
					huh.NewOption("OpenAI", "openai"),
					huh.NewOption("Google Gemini", "gemini"),
					huh.NewOption("Anthropic", "anthropic"),
					huh.NewOption("Other (OpenAI-compatible endpoint)", "openai-compatible"),
				).
				Value(&llmProvider),
		).Title("LLM Provider"),

		huh.NewGroup(
			huh.NewInput().
				Title("API Key").
				DescriptionFunc(func() string {
					if llmProvider == "ollama" {
						return "Ollama runs locally — no API key needed.\nPress Enter to continue."
					}
					return "Your API key for the selected provider.\nLeave empty to configure later."
				}, &llmProvider).
				EchoMode(huh.EchoModePassword).
				PlaceholderFunc(func() string {
					if llmProvider == "ollama" {
						return "not required — press Enter"
					}
					return "paste key here or press Enter to skip"
				}, &llmProvider).
				Value(&llmKey),

			huh.NewInput().
				Title("Model").
				DescriptionFunc(func() string {
					if llmProvider == "ollama" {
						return "The Ollama model for chat. Make sure it's pulled: ollama pull " + defaultChatModel(llmProvider)
					}
					return "The model used to generate AI responses."
				}, &llmProvider).
				PlaceholderFunc(func() string {
					return defaultChatModel(llmProvider)
				}, &llmProvider).
				Value(&llmModel),
		).Title("LLM Settings"),
	).WithTheme(ravenTheme())

	if err := form.Run(); err != nil {
		return err
	}

	if llmModel == "" {
		llmModel = defaultChatModel(llmProvider)
	}
	embeddingModel = defaultEmbeddingModel(llmProvider)

	var salt []byte
	err := spinner.New().
		Title("Deriving encryption key...").
		Action(func() {
			var genErr error
			salt, genErr = crypto.GenerateSalt()
			if genErr != nil {
				return
			}
			_, genErr = crypto.DeriveKey([]byte(password), salt)
			if genErr != nil {
				salt = nil
			}
		}).
		Run()
	if err != nil {
		return fmt.Errorf("key derivation spinner: %w", err)
	}
	if salt == nil {
		return fmt.Errorf("failed to derive encryption key")
	}

	ownerID := make([]byte, 8)
	rand.Read(ownerID)

	cfg.OwnerID = fmt.Sprintf("owner:%x", ownerID)
	cfg.EncryptionSalt = salt
	cfg.TelegramToken = telegramTok
	cfg.LLMProvider = llmProvider
	cfg.LLMAPIKey = llmKey
	cfg.LLMModel = llmModel
	cfg.EmbeddingModel = embeddingModel

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	printBanner("setup complete")
	printOK("Config saved to %s", config.ConfigPath(cfg))
	printOK("Data directory: %s", cfg.DataDir)
	fmt.Println()

	needsAttention := false
	if cfg.TelegramToken == "" {
		printWarn("Telegram token not set — add later via config or RAVENSYNC_TELEGRAM_TOKEN")
		needsAttention = true
	}
	if cfg.LLMAPIKey == "" && cfg.LLMProvider != "ollama" {
		printWarn("API key not set — add later via config or RAVENSYNC_LLM_KEY")
		needsAttention = true
	}
	if cfg.LLMProvider == "ollama" {
		printNote("Using Ollama locally — make sure models are pulled:")
		printLine("    ollama pull %s", cfg.LLMModel)
		printLine("    ollama pull %s", cfg.EmbeddingModel)
	}

	fmt.Println()
	if needsAttention {
		printLine("When ready: %s", cliHighlight.Render("ravensync serve"))
	} else {
		printLine("All set! Start with: %s", cliHighlight.Render("ravensync serve"))
	}

	return nil
}

func applyInitFlags(cmd *cobra.Command, telegramTok, llmProvider, llmKey, llmModel *string) {
	if v, _ := cmd.Flags().GetString("telegram-token"); v != "" {
		*telegramTok = v
	}
	if v, _ := cmd.Flags().GetString("llm-provider"); v != "" {
		*llmProvider = v
	}
	if v, _ := cmd.Flags().GetString("llm-key"); v != "" {
		*llmKey = v
	}
	if v, _ := cmd.Flags().GetString("llm-model"); v != "" {
		*llmModel = v
	}
}

func defaultChatModel(provider string) string {
	switch provider {
	case "ollama":
		return "llama3"
	case "gemini":
		return "gemini-2.0-flash"
	case "anthropic":
		return "claude-sonnet-4-20250514"
	default:
		return "gpt-4o-mini"
	}
}

func defaultEmbeddingModel(provider string) string {
	switch provider {
	case "ollama":
		return "nomic-embed-text"
	case "gemini":
		return "text-embedding-004"
	default:
		return "text-embedding-3-small"
	}
}
