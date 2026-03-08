package sshbbs

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	wish "github.com/charmbracelet/wish"
	wishbubbletea "github.com/charmbracelet/wish/bubbletea"
	"github.com/manuel/wesen/qemu-go-init/internal/bbsapp"
	"github.com/manuel/wesen/qemu-go-init/internal/bbsstore"
	"github.com/muesli/termenv"
)

func Middleware(store *bbsstore.Store) wish.Middleware {
	return wishbubbletea.MiddlewareWithProgramHandler(func(sess ssh.Session) *tea.Program {
		subtitle := fmt.Sprintf("SSH BBS for %s", fallback(sess.User(), "anonymous"))
		model, err := bbsapp.New(store, bbsapp.Options{
			Title:         "qemu-go-init bbs",
			Subtitle:      subtitle,
			StateRoot:     store.Root(),
			DefaultAuthor: fallback(sess.User(), "anonymous"),
		})
		options := append([]tea.ProgramOption{tea.WithAltScreen()}, wishbubbletea.MakeOptions(sess)...)
		if err != nil {
			return tea.NewProgram(errorModel{err: err}, options...)
		}
		program := tea.NewProgram(model, options...)
		model.AttachProgram(sess.Context(), program)
		go func() {
			<-sess.Context().Done()
			_ = model.Close()
		}()
		return program
	}, termenv.Ascii)
}

func fallback(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

type errorModel struct {
	err error
}

func (m errorModel) Init() tea.Cmd {
	return nil
}

func (m errorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c", "q", "enter", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m errorModel) View() string {
	return fmt.Sprintf("failed to start BBS JavaScript REPL:\n\n%v\n\nPress q, enter, esc, or ctrl+c to exit.\n", m.err)
}
