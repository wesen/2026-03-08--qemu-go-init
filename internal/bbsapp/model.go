package bbsapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/manuel/wesen/qemu-go-init/internal/bbsstore"
)

type Store interface {
	ListMessages(ctx context.Context) ([]bbsstore.Message, error)
	CreateMessage(ctx context.Context, params bbsstore.CreateMessageParams) (bbsstore.Message, error)
}

type Options struct {
	Title         string
	Subtitle      string
	StateRoot     string
	DefaultAuthor string
}

type mode int

const (
	modeBrowse mode = iota
	modeCompose
)

type messagesLoadedMsg struct {
	messages []bbsstore.Message
	err      error
}

type messageCreatedMsg struct {
	message bbsstore.Message
	err     error
}

type Model struct {
	store         Store
	title         string
	subtitle      string
	stateRoot     string
	mode          mode
	messages      []bbsstore.Message
	cursor        int
	width         int
	height        int
	status        string
	lastRefreshed string
	author        textinput.Model
	subject       textinput.Model
	body          textarea.Model
	focusIndex    int
}

func New(store Store, options Options) Model {
	author := textinput.New()
	author.Prompt = "author> "
	author.Placeholder = "anonymous"
	author.CharLimit = 48
	author.SetValue(strings.TrimSpace(options.DefaultAuthor))

	subject := textinput.New()
	subject.Prompt = "subject> "
	subject.Placeholder = "post subject"
	subject.CharLimit = 96

	body := textarea.New()
	body.Placeholder = "Write a message. Ctrl+S saves, Esc cancels."
	body.Focus()
	body.SetHeight(10)

	model := Model{
		store:         store,
		title:         fallback(options.Title, "qemu-go-init bbs"),
		subtitle:      fallback(options.Subtitle, "Shared-state Bubble Tea board"),
		stateRoot:     options.StateRoot,
		mode:          modeBrowse,
		status:        "Loading messages...",
		lastRefreshed: "never",
		author:        author,
		subject:       subject,
		body:          body,
	}
	model.applyFocus()
	return model
}

func (m Model) Init() tea.Cmd {
	return loadMessagesCmd(m.store)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
		m.body.SetWidth(max(24, typed.Width-12))
		m.body.SetHeight(max(8, typed.Height-16))
		return m, nil
	case messagesLoadedMsg:
		if typed.err != nil {
			m.status = fmt.Sprintf("reload failed: %v", typed.err)
			return m, nil
		}
		m.messages = typed.messages
		if m.cursor >= len(m.messages) {
			m.cursor = max(0, len(m.messages)-1)
		}
		m.lastRefreshed = time.Now().UTC().Format(time.RFC3339)
		m.status = fmt.Sprintf("Loaded %d messages", len(m.messages))
		return m, nil
	case messageCreatedMsg:
		if typed.err != nil {
			m.status = fmt.Sprintf("save failed: %v", typed.err)
			return m, nil
		}
		m.messages = append([]bbsstore.Message{typed.message}, m.messages...)
		m.cursor = 0
		m.status = "Message posted"
		m.lastRefreshed = time.Now().UTC().Format(time.RFC3339)
		m.mode = modeBrowse
		m.resetComposer()
		return m, nil
	case tea.KeyMsg:
		if m.mode == modeCompose {
			return m.updateCompose(typed)
		}
		return m.updateBrowse(typed)
	default:
		if m.mode == modeCompose {
			return m.updateComposeWidgets(msg)
		}
		return m, nil
	}
}

func (m Model) View() string {
	if m.width == 0 {
		m.width = 100
	}
	if m.height == 0 {
		m.height = 32
	}

	header := m.headerView()
	footer := m.footerView()

	if m.mode == modeCompose {
		content := m.composeView()
		return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
	}

	content := m.browseView()
	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

func (m Model) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.messages)-1 {
			m.cursor++
		}
	case "n":
		m.mode = modeCompose
		m.status = "Compose a new message"
		m.applyFocus()
	case "r":
		m.status = "Reloading messages..."
		return m, loadMessagesCmd(m.store)
	}
	return m, nil
}

func (m Model) updateCompose(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = modeBrowse
		m.status = "Compose cancelled"
		m.resetComposer()
		return m, nil
	case "tab", "shift+tab":
		if msg.String() == "tab" {
			m.focusIndex = (m.focusIndex + 1) % 3
		} else {
			m.focusIndex = (m.focusIndex + 2) % 3
		}
		m.applyFocus()
		return m, nil
	case "ctrl+s":
		m.status = "Saving message..."
		return m, createMessageCmd(m.store, bbsstore.CreateMessageParams{
			Author:  m.author.Value(),
			Subject: m.subject.Value(),
			Body:    m.body.Value(),
		})
	}
	return m.updateComposeWidgets(msg)
}

func (m Model) updateComposeWidgets(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch m.focusIndex {
	case 0:
		var cmd tea.Cmd
		m.author, cmd = m.author.Update(msg)
		cmds = append(cmds, cmd)
	case 1:
		var cmd tea.Cmd
		m.subject, cmd = m.subject.Update(msg)
		cmds = append(cmds, cmd)
	case 2:
		var cmd tea.Cmd
		m.body, cmd = m.body.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) applyFocus() {
	m.author.Blur()
	m.subject.Blur()
	m.body.Blur()

	switch m.focusIndex {
	case 0:
		m.author.Focus()
	case 1:
		m.subject.Focus()
	case 2:
		m.body.Focus()
	}
}

func (m *Model) resetComposer() {
	m.focusIndex = 0
	m.subject.SetValue("")
	m.body.SetValue("")
	m.applyFocus()
}

func (m Model) headerView() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render(m.title)
	subtitle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(m.subtitle)
	meta := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(fmt.Sprintf("state root: %s", fallback(m.stateRoot, "unknown")))
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderBottom(true).
		Padding(0, 1).
		Render(lipgloss.JoinVertical(lipgloss.Left, title, subtitle, meta))
}

func (m Model) browseView() string {
	listWidth := max(28, m.width/3)
	detailWidth := max(36, m.width-listWidth-4)

	var rows []string
	for i, message := range m.messages {
		prefix := "  "
		style := lipgloss.NewStyle().Padding(0, 1)
		if i == m.cursor {
			prefix = "> "
			style = style.Foreground(lipgloss.Color("230")).Background(lipgloss.Color("24"))
		}
		rows = append(rows, style.Render(fmt.Sprintf("%s%s\n   %s", prefix, message.Subject, compactMeta(message))))
	}
	if len(rows) == 0 {
		rows = append(rows, "No messages yet.")
	}

	selected := m.selectedMessage()
	detail := "Select a message."
	if selected != nil {
		detail = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render(selected.Subject),
			lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(compactMeta(*selected)),
			"",
			selected.Body,
		)
	}

	left := lipgloss.NewStyle().Width(listWidth).PaddingRight(1).Render(strings.Join(rows, "\n"))
	right := lipgloss.NewStyle().Width(detailWidth).BorderLeft(true).PaddingLeft(1).Render(detail)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m Model) composeView() string {
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Tab to move focus. Ctrl+S saves. Esc cancels.")
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render("New message"),
			hint,
			"",
			m.author.View(),
			m.subject.View(),
			m.body.View(),
		))
	return box
}

func (m Model) footerView() string {
	keys := "browse: j/k move, n new, r reload, q quit"
	if m.mode == modeCompose {
		keys = "compose: tab focus, ctrl+s save, esc cancel, ctrl+c quit"
	}
	status := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(m.status)
	refresh := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(fmt.Sprintf("last refresh: %s", m.lastRefreshed))
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderTop(true).
		Padding(0, 1).
		Render(lipgloss.JoinVertical(lipgloss.Left, status, refresh, keys))
}

func (m Model) selectedMessage() *bbsstore.Message {
	if len(m.messages) == 0 || m.cursor < 0 || m.cursor >= len(m.messages) {
		return nil
	}
	return &m.messages[m.cursor]
}

func loadMessagesCmd(store Store) tea.Cmd {
	return func() tea.Msg {
		messages, err := store.ListMessages(context.Background())
		return messagesLoadedMsg{messages: messages, err: err}
	}
}

func createMessageCmd(store Store, params bbsstore.CreateMessageParams) tea.Cmd {
	return func() tea.Msg {
		message, err := store.CreateMessage(context.Background(), params)
		return messageCreatedMsg{message: message, err: err}
	}
}

func compactMeta(message bbsstore.Message) string {
	return fmt.Sprintf("%s • %s", message.Author, message.CreatedAt.Local().Format("2006-01-02 15:04"))
}

func fallback(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
