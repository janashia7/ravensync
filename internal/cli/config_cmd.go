package cli

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/ravensync/ravensync/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect or update Ravensync configuration",
	Long: `Interactive menu when run alone (same style as ravensync init).

Subcommands:
  config show        Summary (interactive panel; secrets redacted)
  config set         Wizard to change LLM / Telegram (Enter keeps each field)
  config allow-users Wizard for allowlist (or pass a list / --clear as flags)

Non-interactive (scripts): pass flags to config set or a list to allow-users.`,
	RunE: runConfigMenu,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "View configuration (interactive summary; secrets redacted)",
	RunE:  runConfigShow,
}

var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Update provider, models, or API keys",
	Long: `Interactive wizard when run with no flags (same UI as init; Enter keeps each field).

Flags update only what you pass (non-interactive, for scripts):

  ravensync config set --llm-provider ollama --llm-model llama3
  ravensync config set --llm-key ""
  ravensync config set --telegram-token "123:ABC..."

Does not change encryption_salt or memory.db. Restart ravensync serve after changes.`,
	RunE: runConfigSet,
}

var configAllowUsersCmd = &cobra.Command{
	Use:   "allow-users [LIST]",
	Short: "Set Telegram allowlist",
	Long: `Interactive when run with no arguments and without --clear (same style as init).

  ravensync config allow-users                     # wizard; Enter on unchanged line keeps list
  ravensync config allow-users "123,@youruser"   # non-interactive
  ravensync config allow-users --clear             # allow everyone`,
	Args: cobra.MaximumNArgs(1),
	RunE: runConfigAllowUsers,
}

var configAllowUsersClear bool

func init() {
	configAllowUsersCmd.Flags().BoolVar(&configAllowUsersClear, "clear", false, "Allow all Telegram users (remove whitelist)")
	configSetCmd.Flags().String("telegram-token", "", "Telegram bot token")
	configSetCmd.Flags().String("llm-provider", "", "LLM provider (e.g. openai, ollama)")
	configSetCmd.Flags().String("llm-key", "", "LLM API key (empty string clears it)")
	configSetCmd.Flags().String("llm-model", "", "Chat / completion model name")
	configSetCmd.Flags().String("embedding-model", "", "Embedding model name")

	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configAllowUsersCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigMenu(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown argument %q — use a subcommand (show, set, allow-users) or run: ravensync config", args[0])
	}

	var choice string
	err := huh.NewSelect[string]().
		Title("Ravensync configuration").
		Description("Choose what to do. Encryption and memories are never changed here.").
		Options(
			huh.NewOption("View current configuration", "show"),
			huh.NewOption("Edit LLM, models, and Telegram token", "set"),
			huh.NewOption("Edit Telegram allowlist", "allow"),
			huh.NewOption("Exit", "exit"),
		).
		Value(&choice).
		WithTheme(ravenTheme()).
		Run()
	if err != nil {
		return err
	}

	switch choice {
	case "show":
		return runConfigShow(cmd, nil)
	case "set":
		return runConfigSetInteractive()
	case "allow":
		return runConfigAllowUsersInteractive()
	default:
		return nil
	}
}

func formatAllowlist(cfg *config.Config) string {
	var parts []string
	for _, id := range cfg.AllowedUsers {
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	for _, n := range cfg.AllowedUsernames {
		parts = append(parts, "@"+n)
	}
	return strings.Join(parts, ", ")
}

func allowlistEqual(cfg *config.Config, ids []int64, names []string) bool {
	aID := slices.Clone(cfg.AllowedUsers)
	bID := slices.Clone(ids)
	slices.Sort(aID)
	slices.Sort(bID)
	if !slices.Equal(aID, bID) {
		return false
	}
	a := slices.Clone(cfg.AllowedUsernames)
	b := slices.Clone(names)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	desc := buildConfigSummaryText(cfg)
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Current configuration").
				Description(desc),
		),
	).WithTheme(ravenTheme())

	if err := form.Run(); err != nil {
		return err
	}
	fmt.Println()
	return nil
}

func buildConfigSummaryText(cfg *config.Config) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Path: %s\n", config.ConfigPath(cfg)))
	b.WriteString(fmt.Sprintf("data_dir: %s\n", cfg.DataDir))
	b.WriteString(fmt.Sprintf("llm_provider: %s\n", cfg.LLMProvider))
	b.WriteString(fmt.Sprintf("llm_model: %s\n", cfg.LLMModel))
	b.WriteString(fmt.Sprintf("embedding_model: %s\n", cfg.EmbeddingModel))
	b.WriteString(fmt.Sprintf("telegram_token: %s\n", redactSecret(cfg.TelegramToken)))
	b.WriteString(fmt.Sprintf("llm_api_key: %s\n", redactSecret(cfg.LLMAPIKey)))
	if len(cfg.AllowedUsers) > 0 {
		b.WriteString(fmt.Sprintf("allowed_users: %v\n", cfg.AllowedUsers))
	}
	if len(cfg.AllowedUsernames) > 0 {
		b.WriteString(fmt.Sprintf("allowed_usernames: %v\n", cfg.AllowedUsernames))
	}
	if len(cfg.AllowedUsers) == 0 && len(cfg.AllowedUsernames) == 0 {
		b.WriteString("allowlist: (everyone)\n")
	}
	if len(cfg.EncryptionSalt) > 0 {
		b.WriteString(fmt.Sprintf("encryption_salt: (%d bytes, not editable here)\n", len(cfg.EncryptionSalt)))
	} else {
		b.WriteString("encryption_salt: MISSING — run ravensync init\n")
	}
	b.WriteString("\nPress Enter to close.")
	return b.String()
}

func redactSecret(s string) string {
	if s == "" {
		return "(not set)"
	}
	return "(set, " + fmt.Sprintf("%d", len(s)) + " chars)"
}

func anyConfigSetFlagChanged(cmd *cobra.Command) bool {
	for _, name := range []string{"telegram-token", "llm-provider", "llm-key", "llm-model", "embedding-model"} {
		if cmd.Flags().Changed(name) {
			return true
		}
	}
	return false
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	if !anyConfigSetFlagChanged(cmd) {
		return runConfigSetInteractive()
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	var changed []string
	if cmd.Flags().Changed("telegram-token") {
		v, _ := cmd.Flags().GetString("telegram-token")
		cfg.TelegramToken = strings.TrimSpace(v)
		changed = append(changed, "telegram_token")
	}
	if cmd.Flags().Changed("llm-provider") {
		v, _ := cmd.Flags().GetString("llm-provider")
		cfg.LLMProvider = strings.TrimSpace(v)
		if cfg.LLMProvider == "" {
			return fmt.Errorf("--llm-provider cannot be empty")
		}
		changed = append(changed, "llm_provider")
	}
	if cmd.Flags().Changed("llm-key") {
		v, _ := cmd.Flags().GetString("llm-key")
		cfg.LLMAPIKey = v
		changed = append(changed, "llm_api_key")
	}
	if cmd.Flags().Changed("llm-model") {
		v, _ := cmd.Flags().GetString("llm-model")
		cfg.LLMModel = strings.TrimSpace(v)
		if cfg.LLMModel == "" {
			return fmt.Errorf("--llm-model cannot be empty")
		}
		changed = append(changed, "llm_model")
	}
	if cmd.Flags().Changed("embedding-model") {
		v, _ := cmd.Flags().GetString("embedding-model")
		cfg.EmbeddingModel = strings.TrimSpace(v)
		if cfg.EmbeddingModel == "" {
			return fmt.Errorf("--embedding-model cannot be empty")
		}
		changed = append(changed, "embedding_model")
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	printBanner("config")
	printOK("Updated: %s", strings.Join(changed, ", "))
	printLine("Restart %s to apply.", cliHighlight.Render("ravensync serve"))
	fmt.Println()
	return nil
}

func runConfigSetInteractive() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	var (
		telegramNew   string
		llmProvider   = cfg.LLMProvider
		llmKeyNew     string
		clearLLMKey   bool
		llmModel      = cfg.LLMModel
		embeddingModel = cfg.EmbeddingModel
	)

	providerOpts := []huh.Option[string]{
		huh.NewOption("Ollama (local, free)", "ollama"),
		huh.NewOption("OpenAI", "openai"),
		huh.NewOption("Google Gemini", "gemini"),
		huh.NewOption("Anthropic", "anthropic"),
		huh.NewOption("Other (OpenAI-compatible endpoint)", "openai-compatible"),
	}
	if !providerValueKnown(llmProvider) {
		providerOpts = append([]huh.Option[string]{
			huh.NewOption("Keep current provider ("+llmProvider+")", llmProvider),
		}, providerOpts...)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Edit configuration").
				Description("Encryption salt and memories are not changed.\nLeave fields as-is and press Enter to keep the current value."),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Telegram bot token").
				Description("Leave empty to keep the current token. Paste a new token to replace it.").
				EchoMode(huh.EchoModePassword).
				Placeholder("(unchanged if empty)").
				Value(&telegramNew),
		).Title("Telegram"),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("LLM provider").
				Description("Choose the provider; current value is pre-selected.").
				Options(providerOpts...).
				Value(&llmProvider),
		).Title("LLM provider"),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Clear LLM API key?").
				Description("Turn on to remove the stored key from config (e.g. for Ollama-only). Off keeps or uses the field below.").
				Affirmative("Yes, clear key").
				Negative("No").
				Value(&clearLLMKey),

			huh.NewInput().
				Title("LLM API key").
				Description("Leave empty to keep the current key. Paste to replace.\nIf you chose Clear above, this field is ignored. Ollama usually needs no key.").
				EchoMode(huh.EchoModePassword).
				Placeholder("(unchanged if empty)").
				Value(&llmKeyNew),
		).Title("API key"),
		huh.NewGroup(
			huh.NewInput().
				Title("Chat model").
				Description("Edit or press Enter to keep.").
				Value(&llmModel),

			huh.NewInput().
				Title("Embedding model").
				Description("Edit or press Enter to keep.").
				Value(&embeddingModel),
		).Title("Models"),
	).WithTheme(ravenTheme())

	if err := form.Run(); err != nil {
		return err
	}

	var changed []string

	if strings.TrimSpace(telegramNew) != "" {
		cfg.TelegramToken = strings.TrimSpace(telegramNew)
		changed = append(changed, "telegram_token")
	}

	if llmProvider != cfg.LLMProvider {
		cfg.LLMProvider = llmProvider
		changed = append(changed, "llm_provider")
	}

	if clearLLMKey {
		if cfg.LLMAPIKey != "" {
			cfg.LLMAPIKey = ""
			changed = append(changed, "llm_api_key (cleared)")
		}
	} else if strings.TrimSpace(llmKeyNew) != "" {
		cfg.LLMAPIKey = llmKeyNew
		changed = append(changed, "llm_api_key")
	}

	if strings.TrimSpace(llmModel) != cfg.LLMModel {
		if strings.TrimSpace(llmModel) == "" {
			return fmt.Errorf("chat model cannot be empty")
		}
		cfg.LLMModel = strings.TrimSpace(llmModel)
		changed = append(changed, "llm_model")
	}

	if strings.TrimSpace(embeddingModel) != cfg.EmbeddingModel {
		if strings.TrimSpace(embeddingModel) == "" {
			return fmt.Errorf("embedding model cannot be empty")
		}
		cfg.EmbeddingModel = strings.TrimSpace(embeddingModel)
		changed = append(changed, "embedding_model")
	}

	if len(changed) == 0 {
		printBanner("config")
		printNote("No changes — all values left as-is.")
		fmt.Println()
		return nil
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	printBanner("config")
	printOK("Updated: %s", strings.Join(changed, ", "))
	printLine("Restart %s to apply.", cliHighlight.Render("ravensync serve"))
	fmt.Println()
	return nil
}

func providerValueKnown(p string) bool {
	switch p {
	case "ollama", "openai", "gemini", "anthropic", "openai-compatible":
		return true
	default:
		return false
	}
}

func runConfigAllowUsers(cmd *cobra.Command, args []string) error {
	if configAllowUsersClear {
		return runConfigAllowUsersNonInteractiveClear()
	}
	if len(args) > 0 {
		return runConfigAllowUsersNonInteractiveArgs(args[0])
	}
	return runConfigAllowUsersInteractive()
}

func runConfigAllowUsersNonInteractiveClear() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	cfg.AllowedUsers = nil
	cfg.AllowedUsernames = nil
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	printBanner("config")
	printOK("Allowlist cleared — bot will respond to all Telegram users")
	fmt.Println()
	return nil
}

func runConfigAllowUsersNonInteractiveArgs(list string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	ids, names, err := parseTelegramAllowlist(list)
	if err != nil {
		return err
	}
	cfg.AllowedUsers = ids
	cfg.AllowedUsernames = names
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	printBanner("config")
	if len(ids) > 0 {
		printOK("allowed_users: %v", ids)
	}
	if len(names) > 0 {
		printOK("allowed_usernames: %v", names)
	}
	if len(ids) == 0 && len(names) == 0 {
		printNote("Empty list — bot will respond to all Telegram users")
	}
	printLine("Restart %s for changes to take effect.", cliHighlight.Render("ravensync serve"))
	fmt.Println()
	return nil
}

func runConfigAllowUsersInteractive() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	allowlistStr := formatAllowlist(cfg)
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Telegram allowlist").
				Description("Comma-separated numeric IDs and/or @usernames.\nLeave the line unchanged and press Enter to keep the current list.\nClear the field completely to allow everyone."),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Allowed users").
				Placeholder("e.g. 123456789, @yourusername").
				Validate(func(s string) error {
					_, _, err := parseTelegramAllowlist(s)
					return err
				}).
				Value(&allowlistStr),
		),
	).WithTheme(ravenTheme())

	if err := form.Run(); err != nil {
		return err
	}

	ids, names, err := parseTelegramAllowlist(allowlistStr)
	if err != nil {
		return err
	}

	if allowlistEqual(cfg, ids, names) {
		printBanner("config")
		printNote("Allowlist unchanged.")
		fmt.Println()
		return nil
	}

	cfg.AllowedUsers = ids
	cfg.AllowedUsernames = names
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	printBanner("config")
	if len(ids) > 0 {
		printOK("allowed_users: %v", ids)
	}
	if len(names) > 0 {
		printOK("allowed_usernames: %v", names)
	}
	if len(ids) == 0 && len(names) == 0 {
		printNote("Empty list — bot will respond to all Telegram users")
	}
	printLine("Restart %s for changes to take effect.", cliHighlight.Render("ravensync serve"))
	fmt.Println()
	return nil
}
