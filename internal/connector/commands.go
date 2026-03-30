package connector

import (
	"fmt"
	"strings"
	"time"

	"github.com/ravensync/ravensync/internal/memory"
	"github.com/ravensync/ravensync/internal/metrics"
)

type BotCommand struct {
	Name string
	Args string
}

func ParseCommand(text string) *BotCommand {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return nil
	}

	// Strip @botname suffix (e.g. "/start@MyBot")
	parts := strings.SplitN(text, " ", 2)
	name := strings.SplitN(parts[0], "@", 2)[0]

	var args string
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}
	return &BotCommand{Name: name, Args: args}
}

type CommandContext struct {
	UserID    string
	Store     *memory.Store
	Metrics   *metrics.Collector
	StartTime time.Time
}

func DispatchCommand(cmd *BotCommand, cc CommandContext) string {
	switch cmd.Name {
	case "/start":
		return cmdStart()
	case "/help":
		return cmdHelp()
	case "/stats":
		return cmdStats(cc)
	case "/forget":
		return cmdForget(cc)
	case "/memories":
		return cmdMemories(cc)
	default:
		return fmt.Sprintf("Unknown command: %s\nType /help for available commands.", cmd.Name)
	}
}

func cmdStart() string {
	return "Welcome to Ravensync!\n\n" +
		"I'm your personal AI assistant with encrypted, persistent memory. " +
		"Everything you tell me is stored locally on your device — never on external servers.\n\n" +
		"Just send me a message to get started, or type /help for commands."
}

func cmdHelp() string {
	return "Available commands:\n\n" +
		"/start — Welcome message\n" +
		"/help — Show this help\n" +
		"/stats — Usage statistics\n" +
		"/memories — Show your recent memories\n" +
		"/forget — Delete all your memories\n\n" +
		"Or just send a message to chat!"
}

func cmdStats(cc CommandContext) string {
	summary := cc.Metrics.Summary()
	userMems := cc.Store.CountUser(cc.UserID)
	totalMems := cc.Store.Count()
	uptime := time.Since(cc.StartTime).Round(time.Second)

	return fmt.Sprintf(
		"Ravensync Stats\n\n"+
			"Uptime: %s\n"+
			"Your memories: %d\n"+
			"Total memories: %d\n"+
			"Session: %s",
		uptime, userMems, totalMems, summary,
	)
}

func cmdForget(cc CommandContext) string {
	count := cc.Store.CountUser(cc.UserID)
	if count == 0 {
		return "You have no memories to forget."
	}
	if err := cc.Store.DeleteUser(cc.UserID); err != nil {
		return fmt.Sprintf("Failed to delete memories: %v", err)
	}
	return fmt.Sprintf("Deleted %d memories. Fresh start!", count)
}

func cmdMemories(cc CommandContext) string {
	count := cc.Store.CountUser(cc.UserID)
	if count == 0 {
		return "No memories stored yet. Send me a message to start building memory!"
	}

	recent := cc.Store.ListRecent(cc.UserID, 5)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("You have %d memories. Most recent:\n\n", count))

	for i, m := range recent {
		preview := m.Content
		if len(preview) > 120 {
			preview = preview[:120] + "..."
		}
		preview = strings.ReplaceAll(preview, "\n", " ")
		sb.WriteString(fmt.Sprintf("%d. %s\n   %s\n\n", i+1, m.CreatedAt.Format("Jan 2 15:04"), preview))
	}

	return sb.String()
}
