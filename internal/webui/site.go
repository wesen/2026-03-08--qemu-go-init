package webui

import (
	"context"
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
	"github.com/manuel/wesen/qemu-go-init/internal/sharedstate"
	"github.com/manuel/wesen/qemu-go-init/internal/sshapp"
	"github.com/manuel/wesen/qemu-go-init/internal/storage"
)

//go:embed static/*
var staticFiles embed.FS

type Options struct {
	ListenAddr       string
	Mounts           []boot.MountResult
	Storage          storage.Result
	SharedState      sharedstate.Result
	Network          networking.Result
	Entropy          entropy.Result
	VirtioRNGModule  kmod.Result
	SSHStatus        func() sshapp.Status
	AIChatDebug      func(context.Context) (any, error)
	AIChatHTTPSProbe func(context.Context) (any, error)
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
	Storage         storage.Result     `json:"storage"`
	SharedState     sharedstate.Result `json:"sharedState"`
	Network         networking.Result  `json:"network"`
	Entropy         entropy.Result     `json:"entropy"`
	VirtioRNGModule kmod.Result        `json:"virtioRngModule"`
	SSH             sshapp.Status      `json:"ssh"`
}

func NewHandler(options Options) (http.Handler, error) {
	subtree, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}

	hostname, _ := os.Hostname()
	startedAt := time.Now().UTC()

	status := func() statusResponse {
		sshStatus := sshapp.Status{}
		if options.SSHStatus != nil {
			sshStatus = options.SSHStatus()
		}

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
			Storage:         options.Storage,
			SharedState:     options.SharedState,
			Network:         options.Network,
			Entropy:         options.Entropy,
			VirtioRNGModule: options.VirtioRNGModule,
			SSH:             sshStatus,
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
		writeJSON(w, http.StatusOK, status())
	})
	mux.HandleFunc("/api/debug/aichat/runtime", func(w http.ResponseWriter, r *http.Request) {
		if options.AIChatDebug == nil {
			writeJSON(w, http.StatusNotImplemented, map[string]any{
				"error": "aichat runtime debug is not configured",
			})
			return
		}
		payload, err := options.AIChatDebug(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, payload)
	})
	mux.HandleFunc("/api/debug/aichat/https-probe", func(w http.ResponseWriter, r *http.Request) {
		if options.AIChatHTTPSProbe == nil {
			writeJSON(w, http.StatusNotImplemented, map[string]any{
				"error": "aichat HTTPS probe is not configured",
			})
			return
		}
		payload, err := options.AIChatHTTPSProbe(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, payload)
	})

	return mux, nil
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(statusCode)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(payload)
}
