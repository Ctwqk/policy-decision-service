package telemetry

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

func NewLogger() zerolog.Logger {
	return zerolog.New(os.Stdout).With().Timestamp().Logger().Level(zerolog.InfoLevel)
}

func init() {
	zerolog.TimeFieldFormat = time.RFC3339Nano
}
