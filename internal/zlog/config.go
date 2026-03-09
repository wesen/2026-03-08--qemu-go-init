package zlog

import (
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const envVar = "GO_INIT_ZEROLOG_LEVEL"

func Configure(defaultLevel zerolog.Level) zerolog.Level {
	return ConfigureWithWriter(defaultLevel, os.Stderr)
}

func ConfigureWithWriter(defaultLevel zerolog.Level, writer io.Writer) zerolog.Level {
	level := defaultLevel
	if raw := strings.TrimSpace(os.Getenv(envVar)); raw != "" {
		if parsed, err := zerolog.ParseLevel(raw); err == nil {
			level = parsed
		}
	}
	zerolog.SetGlobalLevel(level)
	if writer == nil {
		writer = os.Stderr
	}
	log.Logger = zerolog.New(writer).With().Timestamp().Logger().Level(level)
	return level
}
