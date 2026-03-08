package webui

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/manuel/wesen/qemu-go-init/internal/boot"
	"github.com/manuel/wesen/qemu-go-init/internal/entropy"
	"github.com/manuel/wesen/qemu-go-init/internal/kmod"
	"github.com/manuel/wesen/qemu-go-init/internal/networking"
)

//go:embed static/*
var staticFiles embed.FS

type Options struct {
	ListenAddr      string
	Mounts          []boot.MountResult
	Network         networking.Result
	Entropy         entropy.Result
	VirtioRNGModule kmod.Result
}

type statusResponse struct {
	PID             int                `json:"pid"`
	Hostname        string             `json:"hostname"`
	GoVersion       string             `json:"goVersion"`
	GOOS            string             `json:"goos"`
	GOARCH          string             `json:"goarch"`
	ListenAddr      string             `json:"listenAddr"`
	StartedAt       string             `json:"startedAt"`
	Uptime          string             `json:"uptime"`
	Mounts          []boot.MountResult `json:"mounts"`
	Network         networking.Result  `json:"network"`
	Entropy         entropy.Result     `json:"entropy"`
	VirtioRNGModule kmod.Result        `json:"virtioRngModule"`
}

func NewHandler(options Options) (http.Handler, error) {
	subtree, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}

	hostname, _ := os.Hostname()
	startedAt := time.Now().UTC()

	status := func() statusResponse {
		return statusResponse{
			PID:             os.Getpid(),
			Hostname:        hostname,
			GoVersion:       runtime.Version(),
			GOOS:            runtime.GOOS,
			GOARCH:          runtime.GOARCH,
			ListenAddr:      options.ListenAddr,
			StartedAt:       startedAt.Format(time.RFC3339),
			Uptime:          time.Since(startedAt).Round(time.Second).String(),
			Mounts:          options.Mounts,
			Network:         options.Network,
			Entropy:         options.Entropy,
			VirtioRNGModule: options.VirtioRNGModule,
		}
	}

	fileserver := http.FileServer(http.FS(subtree))
	mux := http.NewServeMux()
	mux.Handle("/", fileserver)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(status())
	})

	return mux, nil
}
