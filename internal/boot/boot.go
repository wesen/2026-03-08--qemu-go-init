package boot

import (
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

type MountResult struct {
	Source  string `json:"source"`
	Target  string `json:"target"`
	FSType  string `json:"fsType"`
	Data    string `json:"data,omitempty"`
	Mounted bool   `json:"mounted"`
	Error   string `json:"error,omitempty"`
}

type mountSpec struct {
	source string
	target string
	fsType string
	flags  uintptr
	data   string
}

var mountSpecs = []mountSpec{
	{source: "proc", target: "/proc", fsType: "proc"},
	{source: "sysfs", target: "/sys", fsType: "sysfs"},
	{source: "devtmpfs", target: "/dev", fsType: "devtmpfs", data: "mode=0755"},
}

func PrepareFilesystem(logger *log.Logger) []MountResult {
	results := make([]MountResult, 0, len(mountSpecs))
	for _, spec := range mountSpecs {
		results = append(results, mountOne(spec, logger))
	}

	return results
}

func HTTPAddress() string {
	if addr := os.Getenv("GO_INIT_HTTP_ADDR"); addr != "" {
		return addr
	}

	return ":8080"
}

func StartChildReaper(logger *log.Logger) {
	signals := make(chan os.Signal, 8)
	signal.Notify(signals, syscall.SIGCHLD, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for sig := range signals {
			switch sig {
			case syscall.SIGCHLD:
				reapChildren(logger)
			default:
				logger.Printf("received %s; keeping PID 1 alive", sig)
			}
		}
	}()
}

func ServeHTTP(addr string, handler http.Handler, logger *log.Logger) error {
	server := &http.Server{
		Addr:    addr,
		Handler: requestLogger(handler, logger),
	}

	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

func Halt(logger *log.Logger) {
	logger.Printf("entering halt loop to keep PID 1 resident")
	for {
		if err := syscall.Pause(); err != nil {
			logger.Printf("pause returned: %v", err)
		}
	}
}

func requestLogger(next http.Handler, logger *log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Printf("%s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func mountOne(spec mountSpec, logger *log.Logger) MountResult {
	result := MountResult{
		Source: spec.source,
		Target: spec.target,
		FSType: spec.fsType,
		Data:   spec.data,
	}

	if err := os.MkdirAll(spec.target, 0o755); err != nil {
		result.Error = err.Error()
		logger.Printf("mkdir %s failed: %v", spec.target, err)
		return result
	}

	if err := syscall.Mount(spec.source, spec.target, spec.fsType, spec.flags, spec.data); err != nil {
		result.Error = err.Error()
		logger.Printf("mount %s on %s failed: %v", spec.source, spec.target, err)
		return result
	}

	result.Mounted = true
	logger.Printf("mounted %s on %s (%s)", spec.source, spec.target, spec.fsType)
	return result
}

func reapChildren(logger *log.Logger) {
	for {
		var status syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil)
		if pid <= 0 {
			if err != nil && !errors.Is(err, syscall.ECHILD) {
				logger.Printf("wait4 failed: %v", err)
			}
			return
		}

		logger.Printf("reaped child pid=%d status=%#x", pid, status)
	}
}
