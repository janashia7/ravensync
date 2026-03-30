package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/ravensync/ravensync/internal/agent"
	"github.com/ravensync/ravensync/internal/config"
	"github.com/ravensync/ravensync/internal/connector"
	"github.com/ravensync/ravensync/internal/crypto"
	"github.com/ravensync/ravensync/internal/llm"
	"github.com/ravensync/ravensync/internal/memory"
	"github.com/ravensync/ravensync/internal/metrics"
	"github.com/ravensync/ravensync/internal/ui"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Ravensync agent",
	Long:  "Starts the Ravensync runtime with Telegram integration, encrypted memory, and a live TUI dashboard.",
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().String("telegram-token", "", "Telegram bot token (overrides config)")
	serveCmd.Flags().String("llm-key", "", "LLM API key (overrides config)")
	serveCmd.Flags().String("llm-model", "", "LLM model name (overrides config)")
	serveCmd.Flags().String("password", "", "Encryption password (prefer RAVENSYNC_PASSWORD env)")
	serveCmd.Flags().Bool("debug", false, "Enable debug logging")
}

func runServe(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config (run 'ravensync init' first): %w", err)
	}

	applyOverrides(cmd, cfg)

	if cfg.TelegramToken == "" {
		return fmt.Errorf("telegram token required: set in config, --telegram-token flag, or RAVENSYNC_TELEGRAM_TOKEN env")
	}
	if cfg.LLMAPIKey == "" && cfg.LLMProvider != "ollama" {
		return fmt.Errorf("LLM API key required: set in config, --llm-key flag, or RAVENSYNC_LLM_KEY env")
	}

	password, err := resolvePassword(cmd)
	if err != nil {
		return err
	}

	encKey, err := crypto.DeriveKey([]byte(password), cfg.EncryptionSalt)
	if err != nil {
		return fmt.Errorf("derive encryption key: %w", err)
	}

	// Redirect slog to discard so it doesn't corrupt the TUI.
	// All meaningful events go through the EventBus instead.
	logLevel := slog.LevelInfo
	if debug, _ := cmd.Flags().GetBool("debug"); debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: logLevel}))

	dbPath := filepath.Join(cfg.DataDir, "memory.db")
	store, err := memory.NewStore(dbPath, encKey)
	if err != nil {
		return fmt.Errorf("open memory store: %w", err)
	}
	defer func() { _ = store.Close() }()

	var embedder memory.Embedder
	var llmProvider llm.Provider
	if cfg.LLMProvider == "ollama" {
		embedder = memory.NewOllamaEmbedder(cfg.EmbeddingModel)
		llmProvider = llm.NewOllamaProvider(cfg.LLMModel)
	} else {
		embedder = memory.NewOpenAIEmbedder(cfg.LLMAPIKey, cfg.EmbeddingModel)
		llmProvider = llm.NewOpenAIProvider(cfg.LLMAPIKey, cfg.LLMModel)
	}

	collector, err := metrics.NewCollector(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("init metrics: %w", err)
	}
	defer collector.Close()

	events := ui.NewEventBus()

	ag := agent.New(store, embedder, llmProvider, collector, logger, events)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	tg, err := connector.NewTelegramConnector(cfg.TelegramToken, cfg.AllowedUsers, cfg.AllowedUsernames, ag, logger, events)
	if err != nil {
		return fmt.Errorf("create telegram connector: %w", err)
	}

	go func() {
		if err := tg.Start(ctx); err != nil {
			logger.Error("telegram connector stopped", "error", err)
		}
	}()

	events.Emit(ui.Event{
		Type:    ui.EventInfo,
		Message: fmt.Sprintf("ravensync %s started (memories: %d)", Version, store.Count()),
	})

	dashboard := ui.NewDashboard(ui.DashboardConfig{
		ModelName:   cfg.LLMModel,
		Provider:    cfg.LLMProvider,
		MemoryCount: store.Count(),
		EventCh:     events.Subscribe(),
		Handler: func(ctx context.Context, userID, text string) (string, error) {
			return ag.HandleMessage(ctx, userID, text, nil)
		},
		Ctx: ctx,
		Cancel:      cancel,
		LocalUserID: "console",
	})

	p := tea.NewProgram(dashboard, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	collector.Close()
	_ = store.Close()

	return nil
}

func applyOverrides(cmd *cobra.Command, cfg *config.Config) {
	if v, _ := cmd.Flags().GetString("telegram-token"); v != "" {
		cfg.TelegramToken = v
	}
	if v, _ := cmd.Flags().GetString("llm-key"); v != "" {
		cfg.LLMAPIKey = v
	}
	if v, _ := cmd.Flags().GetString("llm-model"); v != "" {
		cfg.LLMModel = v
	}
	if v := os.Getenv("RAVENSYNC_TELEGRAM_TOKEN"); v != "" && cfg.TelegramToken == "" {
		cfg.TelegramToken = v
	}
	if v := os.Getenv("RAVENSYNC_LLM_KEY"); v != "" && cfg.LLMAPIKey == "" {
		cfg.LLMAPIKey = v
	}
}

func resolvePassword(cmd *cobra.Command) (string, error) {
	if p, _ := cmd.Flags().GetString("password"); p != "" {
		return p, nil
	}
	if p := os.Getenv("RAVENSYNC_PASSWORD"); p != "" {
		return p, nil
	}
	fmt.Print("Enter encryption password: ")
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return string(pw), nil
}
