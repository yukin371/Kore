package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

// LogLevel represents the severity level of a log message
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var (
	levelNames = map[LogLevel]string{
		DEBUG: "DEBUG",
		INFO:  "INFO",
		WARN:  "WARN",
		ERROR: "ERROR",
	}
)

// Logger provides structured logging with levels
type Logger struct {
	mu       sync.Mutex
	level    LogLevel
	out      io.Writer
	errOut   io.Writer
	prefix   string
}

// default logger instance
var std = New(os.Stdout, os.Stderr, INFO, "")

// New creates a new logger instance
func New(out, errOut io.Writer, level LogLevel, prefix string) *Logger {
	return &Logger{
		level:  level,
		out:    out,
		errOut: errOut,
		prefix: prefix,
	}
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetPrefix sets the logger prefix
func (l *Logger) SetPrefix(prefix string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prefix = prefix
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// log is the internal logging method
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	var out io.Writer
	if level >= ERROR {
		out = l.errOut
	} else {
		out = l.out
	}

	prefix := l.prefix
	if prefix != "" {
		prefix += " "
	}

	msg := fmt.Sprintf("[%s] %s%s\n", levelNames[level], prefix, fmt.Sprintf(format, args...))
	log.New(out, "", 0).Print(msg)
}

// Package-level convenience functions using the default logger

// SetLevel sets the minimum log level for the default logger
func SetLevel(level LogLevel) {
	std.SetLevel(level)
}

// SetPrefix sets the prefix for the default logger
func SetPrefix(prefix string) {
	std.SetPrefix(prefix)
}

// Debug logs a debug message using the default logger
func Debug(format string, args ...interface{}) {
	std.Debug(format, args...)
}

// Info logs an info message using the default logger
func Info(format string, args ...interface{}) {
	std.Info(format, args...)
}

// Warn logs a warning message using the default logger
func Warn(format string, args ...interface{}) {
	std.Warn(format, args...)
}

// Error logs an error message using the default logger
func Error(format string, args ...interface{}) {
	std.Error(format, args...)
}
