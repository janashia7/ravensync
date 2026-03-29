package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/ravensync/ravensync/internal/config"
	"github.com/ravensync/ravensync/internal/metrics"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show usage statistics from the events log",
	RunE:  runStats,
}

func init() {
	statsCmd.Flags().Int("days", 7, "Number of days to include")
}

func runStats(cmd *cobra.Command, args []string) error {
	cfg := config.DefaultConfig()
	eventsPath := filepath.Join(cfg.DataDir, "events.jsonl")

	f, err := os.Open(eventsPath)
	if err != nil {
		if os.IsNotExist(err) {
			printBanner("stats")
			printNote("No events recorded yet. Run %s first.", cliHighlight.Render("ravensync serve"))
			fmt.Println()
			return nil
		}
		return fmt.Errorf("open events: %w", err)
	}
	defer func() { _ = f.Close() }()

	days, _ := cmd.Flags().GetInt("days")
	cutoff := time.Now().AddDate(0, 0, -days)

	var (
		totalMessages int
		totalErrors   int
		totalLatency  int64
		users         = make(map[string]bool)
		starts        int
	)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var evt metrics.Event
		if err := json.Unmarshal(scanner.Bytes(), &evt); err != nil {
			continue
		}
		ts, err := time.Parse(time.RFC3339, evt.Timestamp)
		if err != nil || ts.Before(cutoff) {
			continue
		}
		switch evt.Type {
		case "message":
			totalMessages++
			totalLatency += evt.LatencyMs
			if evt.UserID != "" {
				users[evt.UserID] = true
			}
		case "message_error":
			totalErrors++
			if evt.UserID != "" {
				users[evt.UserID] = true
			}
		case "daemon_start":
			starts++
		}
	}

	printBanner(fmt.Sprintf("stats (last %d days)", days))
	printLine("Daemon starts:   %s", cliHighlight.Render(fmt.Sprintf("%d", starts)))
	printLine("Messages:        %s", cliHighlight.Render(fmt.Sprintf("%d", totalMessages)))
	printLine("Errors:          %s", cliHighlight.Render(fmt.Sprintf("%d", totalErrors)))
	printLine("Unique users:    %s", cliHighlight.Render(fmt.Sprintf("%d", len(users))))
	if totalMessages > 0 {
		printLine("Avg latency:     %s", cliHighlight.Render(fmt.Sprintf("%dms", totalLatency/int64(totalMessages))))
	}
	fmt.Println()

	return nil
}
