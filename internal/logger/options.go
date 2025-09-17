package logger

import (
	"io"

	"github.com/rs/zerolog"
)

// Config holds logger configuration
type Config struct {
	output       io.Writer
	level        zerolog.Level
	excludeParts []string
	isDev        bool
}

// Option configures the logger
type Option interface {
	apply(*Config)
}

type optionFunc func(*Config)

func (f optionFunc) apply(cfg *Config) {
	f(cfg)
}

// WithLevel sets the logger level
func WithLevel(level string) Option {
	return optionFunc(func(cfg *Config) {
		cfg.level = parseLevel(level)
	})
}

// WithConsoleWriter uses the console for pretty printer
func WithConsoleWriter(isDev bool) Option {
	return optionFunc(func(cfg *Config) {
		cfg.isDev = isDev
	})
}

// WithOutput sets the output writer
func WithOutput(output io.Writer) Option {
	return optionFunc(func(cfg *Config) {
		cfg.output = output
	})
}

func parseLevel(level string) zerolog.Level {
	switch level {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}
