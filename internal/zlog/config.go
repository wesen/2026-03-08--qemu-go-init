package zlog

import (
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const envVar = "GO_INIT_ZEROLOG_LEVEL"

func Configure(defaultLevel zerolog.Level) zerolog.Level {
	level := defaultLevel
	if raw := strings.TrimSpace(os.Getenv(envVar)); raw != "" {
		if parsed, err := zerolog.ParseLevel(raw); err == nil {
			level = parsed
		}
	}
	zerolog.SetGlobalLevel(level)
	log.Logger = log.Level(level)
	return level
}
