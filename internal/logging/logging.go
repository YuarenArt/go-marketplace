package logging

import (
	"github.com/YuarenArt/marketgo/internal/config"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Logger defines the interface for structured logging.
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

func NewLogger(cfg *config.Config) Logger {
	return newSlogLogger(cfg)
}

type SlogLogger struct {
	logger *slog.Logger
}

func newSlogLogger(cfg *config.Config) Logger {
	writer := os.Stdout
	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{})

	return &SlogLogger{
		logger: slog.New(handler),
	}
}

func setupFileWriter(logFile string) io.Writer {
	logDir := filepath.Dir(logFile)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		slog.Error("Failed to create log directory", "error", err, "path", logDir)
		return os.Stdout
	}

	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		slog.Error("Failed to open log file", "error", err, "path", logFile)
		return os.Stdout
	}

	return file
}

func (l *SlogLogger) Debug(msg string, keysAndValues ...interface{}) {
	l.logger.Debug(msg, keysAndValues...)
}

func (l *SlogLogger) Info(msg string, keysAndValues ...interface{}) {
	l.logger.Info(msg, keysAndValues...)
}

func (l *SlogLogger) Warn(msg string, keysAndValues ...interface{}) {
	l.logger.Warn(msg, keysAndValues...)
}

func (l *SlogLogger) Error(msg string, keysAndValues ...interface{}) {
	l.logger.Error(msg, keysAndValues...)
}
