package testutil

import (
	"bytes"
	"io"
	"os"

	"github.com/rs/zerolog"
)

func NewTestLogger() *zerolog.Logger {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	return &logger
}

func NewBufferedLogger() (*zerolog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	consoleWriter := zerolog.ConsoleWriter{
		Out: os.Stdout,
	}
	logger := zerolog.New(io.MultiWriter(consoleWriter, &buf)).With().Timestamp().Logger()
	return &logger, &buf
}
