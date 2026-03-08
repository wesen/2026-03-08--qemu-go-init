package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/charmbracelet/ssh"
	wish "github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/logging"
)

func main() {
	addr := getenv("WISH_PROBE_ADDR", "127.0.0.1:22230")
	workdir := getenv("WISH_PROBE_WORKDIR", ".")
	hostKeyPath := filepath.Join(workdir, ".wish_probe_ed25519")

	server, err := wish.NewServer(
		wish.WithAddress(addr),
		wish.WithHostKeyPath(hostKeyPath),
		wish.WithMiddleware(
			logging.Middleware(),
			activeterm.Middleware(),
			echoMiddleware(),
		),
	)
	if err != nil {
		log.Fatalf("wish probe: create server: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("wish probe: shutdown error: %v", err)
		}
	}()

	log.Printf("wish probe: listening on %s hostkey=%s", addr, hostKeyPath)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Fatalf("wish probe: listen: %v", err)
	}
}

func echoMiddleware() wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			pty, winCh, hasPTY := s.Pty()
			_, _ = io.WriteString(s, "wish probe ready\n")
			_, _ = io.WriteString(s, fmt.Sprintf("user=%s\n", s.User()))
			_, _ = io.WriteString(s, fmt.Sprintf("remote=%s\n", s.RemoteAddr()))
			_, _ = io.WriteString(s, fmt.Sprintf("command=%q\n", s.Command()))
			_, _ = io.WriteString(s, fmt.Sprintf("has_pty=%t\n", hasPTY))
			if hasPTY {
				_, _ = io.WriteString(s, fmt.Sprintf("term=%s size=%dx%d\n", pty.Term, pty.Window.Width, pty.Window.Height))
			}
			select {
			case win := <-winCh:
				_, _ = io.WriteString(s, fmt.Sprintf("resize=%dx%d\n", win.Width, win.Height))
			default:
			}
			_, _ = io.WriteString(s, "goodbye\n")
			if next != nil {
				next(s)
			}
		}
	}
}

func getenv(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
