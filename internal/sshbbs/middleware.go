package sshbbs

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	wish "github.com/charmbracelet/wish"
	wishbubbletea "github.com/charmbracelet/wish/bubbletea"
	"github.com/manuel/wesen/qemu-go-init/internal/bbsapp"
	"github.com/manuel/wesen/qemu-go-init/internal/bbsstore"
)

func Middleware(store *bbsstore.Store) wish.Middleware {
	return wishbubbletea.Middleware(func(sess ssh.Session) (tea.Model, []tea.ProgramOption) {
		subtitle := fmt.Sprintf("SSH BBS for %s", fallback(sess.User(), "anonymous"))
		model := bbsapp.New(store, bbsapp.Options{
			Title:         "qemu-go-init bbs",
			Subtitle:      subtitle,
			StateRoot:     store.Root(),
			DefaultAuthor: fallback(sess.User(), "anonymous"),
		})
		return model, []tea.ProgramOption{tea.WithAltScreen()}
	})
}

func fallback(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
