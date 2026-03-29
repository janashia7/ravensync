package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type MessageHandler func(ctx context.Context, userID, text string) (string, error)

type DashboardConfig struct {
	ModelName   string
	Provider    string
	MemoryCount int
	EventCh     <-chan Event
	Handler     MessageHandler
	Ctx         context.Context
	Cancel      context.CancelFunc
	LocalUserID string
}

type eventMsg Event
type tickMsg time.Time

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type Dashboard struct {
	cfg DashboardConfig

	chatLines   []string
	eventLines  []string
	chat        viewport.Model
	events      viewport.Model
	input       textinput.Model
	width       int
	height      int
	ready       bool
	focus       int // 0=input, 1=chat, 2=events
	startTime   time.Time
	msgCount    int
	userCount   map[string]bool
	memCount    int
	localUserID string
	thinking    bool
	spinFrame   int
}

var (
	crimson  = lipgloss.Color("160") // sharingan red
	darkRed  = lipgloss.Color("88")  // deep blood red
	ember    = lipgloss.Color("131") // muted warm red
	ash      = lipgloss.Color("240") // dark gray
	smoke    = lipgloss.Color("236") // near-black gray
	bone     = lipgloss.Color("252") // light gray text
	scarlet  = lipgloss.Color("196") // bright red for errors

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(crimson)

	dimStyle = lipgloss.NewStyle().
			Foreground(ash)

	userLabelStyle = lipgloss.NewStyle().
			Foreground(bone).
			Bold(true)

	botLabelStyle = lipgloss.NewStyle().
			Foreground(crimson).
			Bold(true)

	thinkStyle = lipgloss.NewStyle().
			Foreground(ember).
			Italic(true)

	errorMsgStyle = lipgloss.NewStyle().
			Foreground(scarlet)

	evtEmbedStyle  = lipgloss.NewStyle().Foreground(ash)
	evtSearchStyle = lipgloss.NewStyle().Foreground(ember)
	evtLLMStyle    = lipgloss.NewStyle().Foreground(crimson)
	evtStoreStyle  = lipgloss.NewStyle().Foreground(darkRed)
	evtSendStyle   = lipgloss.NewStyle().Foreground(ember)
	evtInfoStyle   = lipgloss.NewStyle().Foreground(ash)
	evtErrorStyle  = lipgloss.NewStyle().Foreground(scarlet)

	keyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(bone)

	keyDescStyle = lipgloss.NewStyle().
			Foreground(ash)
)

func NewDashboard(cfg DashboardConfig) Dashboard {
	ti := textinput.New()
	ti.Placeholder = "type a message..."
	ti.Prompt = ""
	ti.Focus()
	ti.CharLimit = 2000

	localUID := cfg.LocalUserID
	if localUID == "" {
		localUID = "local:console"
	}

	return Dashboard{
		cfg:         cfg,
		input:       ti,
		startTime:   time.Now(),
		userCount:   make(map[string]bool),
		memCount:    cfg.MemoryCount,
		localUserID: localUID,
	}
}

func (d Dashboard) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		d.listenEvents(),
		d.tickCmd(),
	)
}

func (d Dashboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			d.cfg.Cancel()
			return d, tea.Quit
		case "tab":
			d.focus = (d.focus + 1) % 3
			if d.focus == 0 {
				d.input.Focus()
			} else {
				d.input.Blur()
			}
		case "enter":
			if d.focus == 0 && !d.thinking {
				text := strings.TrimSpace(d.input.Value())
				if text != "" {
					d.input.SetValue("")
					d.appendChat(d.formatLocalUserMsg(text))
					d.startThinking()
					cmds = append(cmds, d.sendLocalMessage(text))
				}
			}
		}

	case tea.WindowSizeMsg:
		d.width = msg.Width
		d.height = msg.Height
		d.ready = true
		d.recalcLayout()

	case eventMsg:
		evt := Event(msg)
		d.handleEvent(evt)
		cmds = append(cmds, d.listenEvents())

	case tickMsg:
		if d.thinking {
			d.spinFrame = (d.spinFrame + 1) % len(spinnerFrames)
			d.updateThinkingLine()
		}
		cmds = append(cmds, d.tickCmd())

	case localResponseMsg:
		d.stopThinking()
		if msg.err != nil {
			d.appendChat(formatErrorMsg(msg.err.Error()))
		} else {
			d.appendChat(formatBotMsg(msg.response))
			d.appendChat("")
		}
	}

	switch d.focus {
	case 0:
		var cmd tea.Cmd
		d.input, cmd = d.input.Update(msg)
		cmds = append(cmds, cmd)
	case 1:
		var cmd tea.Cmd
		d.chat, cmd = d.chat.Update(msg)
		cmds = append(cmds, cmd)
	default:
		var cmd tea.Cmd
		d.events, cmd = d.events.Update(msg)
		cmds = append(cmds, cmd)
	}

	return d, tea.Batch(cmds...)
}

func (d *Dashboard) startThinking() {
	d.thinking = true
	d.spinFrame = 0
	frame := spinnerFrames[0]
	line := botLabelStyle.Render("Ravensync") + " " + thinkStyle.Render(frame+" thinking...")
	d.chatLines = append(d.chatLines, d.wrapText(line))
	d.chat.SetContent(strings.Join(d.chatLines, "\n"))
	d.chat.GotoBottom()
}

func (d *Dashboard) updateThinkingLine() {
	if len(d.chatLines) == 0 {
		return
	}
	frame := spinnerFrames[d.spinFrame]
	line := botLabelStyle.Render("Ravensync") + " " + thinkStyle.Render(frame+" thinking...")
	d.chatLines[len(d.chatLines)-1] = d.wrapText(line)
	d.chat.SetContent(strings.Join(d.chatLines, "\n"))
	d.chat.GotoBottom()
}

func (d *Dashboard) stopThinking() {
	d.thinking = false
	if len(d.chatLines) > 0 {
		d.chatLines = d.chatLines[:len(d.chatLines)-1]
	}
}

func (d Dashboard) View() string {
	if !d.ready {
		return "Starting Ravensync..."
	}

	header := " " + titleStyle.Render(fmt.Sprintf("Ravensync (%s)", d.cfg.ModelName)) +
		dimStyle.Render(" — Ctrl+C to exit")

	chatColor := smoke
	if d.focus == 1 {
		chatColor = crimson
	}
	chatBox := buildBox("", d.chat.View(), d.width, d.chatHeight(), chatColor)

	inputColor := smoke
	if d.focus == 0 {
		inputColor = crimson
	}
	inputLine := titleStyle.Render("> ") + d.input.View()
	inputBox := buildBox(titleStyle.Render("ravensync >"), inputLine, d.width, 1, inputColor)

	eventsColor := smoke
	if d.focus == 2 {
		eventsColor = darkRed
	}
	eventsBox := buildBox(dimStyle.Render("Events"), d.events.View(), d.width, d.eventsHeight(), eventsColor)

	footer := d.renderFooter()

	return header + "\n" + chatBox + "\n" + inputBox + "\n" + eventsBox + "\n" + footer
}

func buildBox(title, content string, w, h int, bColor lipgloss.Color) string {
	bc := lipgloss.NewStyle().Foreground(bColor)
	innerW := w - 4
	if innerW < 1 {
		innerW = 1
	}

	var top string
	if title != "" {
		titlePart := " " + title + " "
		titleVisW := lipgloss.Width(titlePart)
		dashes := w - 3 - titleVisW
		if dashes < 1 {
			dashes = 1
		}
		top = bc.Render("╭─") + titlePart + bc.Render(strings.Repeat("─", dashes)+"╮")
	} else {
		top = bc.Render("╭" + strings.Repeat("─", w-2) + "╮")
	}

	lines := strings.Split(content, "\n")
	var mid strings.Builder
	for i := 0; i < h; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		visW := lipgloss.Width(line)
		pad := innerW - visW
		if pad < 0 {
			pad = 0
		}
		mid.WriteString(bc.Render("│") + " " + line + strings.Repeat(" ", pad) + " " + bc.Render("│") + "\n")
	}

	bot := bc.Render("╰" + strings.Repeat("─", w-2) + "╯")

	return top + "\n" + mid.String() + bot
}

func (d *Dashboard) renderFooter() string {
	left := " " +
		keyStyle.Render("Tab") + keyDescStyle.Render(" Switch") + "  " +
		keyStyle.Render("Ctrl+C") + keyDescStyle.Render(" Quit")

	uptime := time.Since(d.startTime).Round(time.Second)

	status := "● Ready"
	if d.thinking {
		status = "◌ Thinking"
	}

	right := dimStyle.Render(fmt.Sprintf(
		"%d mem  %d msgs  %s  %s  ",
		d.memCount, d.msgCount, uptime, d.cfg.Provider,
	)) + keyStyle.Render(status) + " "

	gap := d.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}

	return left + strings.Repeat(" ", gap) + right
}

func (d *Dashboard) chatHeight() int {
	fixed := 9
	remaining := d.height - fixed
	if remaining < 4 {
		remaining = 4
	}
	h := remaining * 2 / 3
	if h < 2 {
		h = 2
	}
	return h
}

func (d *Dashboard) eventsHeight() int {
	fixed := 9
	remaining := d.height - fixed
	if remaining < 4 {
		remaining = 4
	}
	h := remaining - d.chatHeight()
	if h < 2 {
		h = 2
	}
	return h
}

func (d *Dashboard) recalcLayout() {
	innerW := d.width - 4

	d.chat = viewport.New(innerW, d.chatHeight())
	d.chat.SetContent(strings.Join(d.chatLines, "\n"))

	d.events = viewport.New(innerW, d.eventsHeight())
	d.events.SetContent(strings.Join(d.eventLines, "\n"))

	d.input.Width = innerW - 4
}

func (d *Dashboard) contentWidth() int {
	w := d.width - 6
	if w < 20 {
		w = 20
	}
	return w
}

func (d *Dashboard) wrapText(text string) string {
	cw := d.contentWidth()
	if cw <= 0 {
		return text
	}
	return lipgloss.NewStyle().Width(cw).Render(text)
}

func (d *Dashboard) appendChat(line string) {
	if line == "" {
		d.chatLines = append(d.chatLines, "")
	} else {
		d.chatLines = append(d.chatLines, d.wrapText(line))
	}
	d.chat.SetContent(strings.Join(d.chatLines, "\n"))
	d.chat.GotoBottom()
}

func (d *Dashboard) appendEvent(line string) {
	d.eventLines = append(d.eventLines, line)
	d.events.SetContent(strings.Join(d.eventLines, "\n"))
	d.events.GotoBottom()
}

func (d *Dashboard) formatLocalUserMsg(text string) string {
	return userLabelStyle.Render("you") + " " + text
}

func formatUserMsgFrom(name, text string) string {
	return userLabelStyle.Render(name) + " " + text
}

func formatBotMsg(text string) string {
	return botLabelStyle.Render("Ravensync") + " " + text
}

func formatErrorMsg(text string) string {
	return errorMsgStyle.Render("✗ " + text)
}

func (d *Dashboard) handleEvent(evt Event) {
	switch evt.Type {
	case EventMessageIn:
		d.msgCount++
		if evt.UserID != "" {
			d.userCount[evt.UserID] = true
		}
		d.appendChat(formatUserMsgFrom(evt.UserID, evt.Message))
		d.appendEvent(evtInfoStyle.Render(evt.FormatLog()))

	case EventMessageOut:
		if d.thinking {
			d.stopThinking()
		}
		d.appendChat(formatBotMsg(evt.Message))
		d.appendChat("")
		d.appendEvent(evtSendStyle.Render(evt.FormatLog()))

	case EventEmbedding:
		d.appendEvent(evtEmbedStyle.Render(evt.FormatLog()))

	case EventMemorySearch:
		d.appendEvent(evtSearchStyle.Render(evt.FormatLog()))

	case EventLLMCall:
		d.appendEvent(evtLLMStyle.Render(evt.FormatLog()))

	case EventLLMResponse:
		d.appendEvent(evtLLMStyle.Render(evt.FormatLog()))

	case EventMemoryStore:
		d.memCount++
		d.appendEvent(evtStoreStyle.Render(evt.FormatLog()))

	case EventError:
		d.appendEvent(evtErrorStyle.Render(evt.FormatLog()))

	case EventInfo:
		d.appendEvent(evtInfoStyle.Render(evt.FormatLog()))

	default:
		d.appendEvent(dimStyle.Render(evt.FormatLog()))
	}
}

func (d Dashboard) listenEvents() tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-d.cfg.EventCh
		if !ok {
			return nil
		}
		return eventMsg(evt)
	}
}

func (d Dashboard) tickCmd() tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type localResponseMsg struct {
	response string
	err      error
}

func (d Dashboard) sendLocalMessage(text string) tea.Cmd {
	userID := d.localUserID
	return func() tea.Msg {
		resp, err := d.cfg.Handler(d.cfg.Ctx, userID, text)
		return localResponseMsg{response: resp, err: err}
	}
}
