package logger

import (
	"os"

	"github.com/rs/zerolog"
)

const DefaultLogLevel = "info"

// New creates a new logger instance
func New(opts ...Option) *zerolog.Logger {
	// Default config
	config := &Config{
		output:       os.Stdout,
		level:        zerolog.InfoLevel,
		excludeParts: []string{zerolog.TimestampFieldName, zerolog.LevelFieldName},
		isDev:        true,
	}

	// Apply options
	for _, opt := range opts {
		opt.apply(config)
	}

	// Create logger
	logger := zerolog.New(config.output).
		Level(config.level).
		With().
		Logger()

	// Pretty logging for development
	if config.isDev {
		logger = logger.Output(zerolog.ConsoleWriter{
			Out:          config.output,
			PartsExclude: config.excludeParts,
		})
	}

	return &logger
}

func NewConsoleLogger() *zerolog.Logger {
	return New(
		WithLevel(DefaultLogLevel),
		WithOutput(os.Stderr),
		WithConsoleWriter(true),
	)
}
