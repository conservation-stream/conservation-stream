package forward

import (
	"io"
	"log"
)

type Logger interface {
	Printf(format string, args ...any)
	Println(args ...any)
}

type stdLogger struct {
	logger *log.Logger
}

func NewLogger(output io.Writer) Logger {
	return &stdLogger{
		logger: log.New(output, "", 0),
	}
}

func (logger *stdLogger) Printf(format string, args ...any) {
	logger.logger.Printf(format, args...)
}

func (logger *stdLogger) Println(args ...any) {
	logger.logger.Println(args...)
}

type noopLogger struct{}

func NewNoopLogger() Logger {
	return noopLogger{}
}

func (noopLogger) Printf(format string, args ...any) {}

func (noopLogger) Println(args ...any) {}
