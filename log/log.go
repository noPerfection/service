// Package log defines the logger engine.
// The unique feature is that it can create a child logger derived from the parent logger.
// Each logger defines a unique color style for the message outputs.
//
// Create a child logger for the packages that the service is calling.
package log

import (
	"fmt"
	"github.com/charmbracelet/log"

	"github.com/charmbracelet/lipgloss"

	"github.com/muesli/gamut"
)

// Logger is the wrapper over the logger and keeps the style.
// The style is generated randomly.
type Logger struct {
	logger log.Logger
	style  LoggerStyle
}

// LoggerStyle defines the various colors for each log parts.
type LoggerStyle struct {
	prefix    lipgloss.Style
	separator lipgloss.Style
}

func randomStyle() (LoggerStyle, error) {
	rawPalette, err := gamut.Generate(2, gamut.PastelGenerator{})
	if err != nil {
		return LoggerStyle{}, fmt.Errorf("color.Generate: %w", err)
	}
	palette := make([]lipgloss.Color, len(rawPalette))
	for i, rawPalette := range rawPalette {
		lighter := gamut.Lighter(rawPalette, 0.05)
		palette[i] = lipgloss.Color(gamut.ToHex(lighter))
	}

	// web: questions/42480000/python-ansi-colour-codes-transparent-background
	backgroundColor := lipgloss.Color("49m")

	style := LoggerStyle{}

	style.prefix = lipgloss.NewStyle().
		Bold(true).
		Faint(true).
		Background(backgroundColor).
		Foreground(palette[0])

	// SeparatorStyle is the style for separators.
	style.separator = lipgloss.NewStyle().
		Faint(true).
		Background(backgroundColor).
		Foreground(palette[1])

	return style, nil
}

func (style LoggerStyle) setPrimary() LoggerStyle {
	log.PrefixStyle = style.prefix
	log.SeparatorStyle = style.separator

	return style
}

// New logger with the prefix and timestamp.
// It generates the random color style.
func New(prefix string, timestamp bool) (*Logger, error) {
	randomStyle, err := randomStyle()
	if err != nil {
		return nil, fmt.Errorf("random_style: %w", err)
	}

	logger := log.New()
	logger.SetPrefix(prefix)
	logger.SetReportCaller(false)
	logger.SetReportTimestamp(timestamp)

	newLogger := Logger{
		logger: logger,
		style:  randomStyle,
	}

	return &newLogger, nil
}

// Fatal calls the Error, then os.Exit()
func Fatal(title string, kv ...interface{}) {
	log.Fatal(title, kv...)
}

// Info prints the information
func (logger *Logger) Info(title string, kv ...interface{}) {
	logger.style.setPrimary()
	logger.logger.Info(title, kv...)
}

func (logger *Logger) Prefix() string {
	return logger.Prefix()
}

// Fatal prints the error message and then calls the os.Exit()
func (logger *Logger) Fatal(title string, kv ...interface{}) {
	logger.style.setPrimary()
	logger.logger.Fatal(title, kv...)
}

// Warn prints the warning message
func (logger *Logger) Warn(title string, kv ...interface{}) {
	logger.style.setPrimary()
	logger.logger.Warn(title, kv...)
}

// Error prints the error message
func (logger *Logger) Error(title string, kv ...interface{}) {
	logger.style.setPrimary()
	logger.logger.Error(title, kv...)
}

// Child logger from the parent with its own color style.
//
// When to use it?
//
// For example:
//
//	parent, _ := log.New("main", false)
//	db_log, _ := parent.Child("database")
//	reply, _ := parent.Child("controller")
//
//	parent.Info("starting", "security_enabled", true)
//	db_log.Info("starting")
//	reply.Info("starting", "port", 443)
//
//	// prints the following
//	// INFO main: starting: security_enabled=true
//	// INFO main::database: starting
//	// INFO main::controller: starting, port=443
func (logger *Logger) Child(prefix string, kv ...interface{}) *Logger {
	child := logger.logger.With(kv...)
	child.SetReportTimestamp(true)

	child.SetPrefix(logger.logger.GetPrefix() + "/" + prefix)

	return &Logger{
		logger: child,
		style:  logger.style,
	}
}
