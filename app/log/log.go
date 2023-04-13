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

func random_style() (LoggerStyle, error) {
	raw_palette, err := gamut.Generate(2, gamut.PastelGenerator{})
	if err != nil {
		return LoggerStyle{}, fmt.Errorf("color.Generate: %w", err)
	}
	palette := make([]lipgloss.Color, len(raw_palette))
	for i, raw_palette := range raw_palette {
		lighter := gamut.Lighter(raw_palette, 0.05)
		palette[i] = lipgloss.Color(gamut.ToHex(lighter))
	}

	// web: questions/42480000/python-ansi-colour-codes-transparent-background
	background_color := lipgloss.Color("49m")

	style := LoggerStyle{}

	style.prefix = lipgloss.NewStyle().
		Bold(true).
		Faint(true).
		Background(background_color).
		Foreground(palette[0])

	// SeparatorStyle is the style for separators.
	style.separator = lipgloss.NewStyle().
		Faint(true).
		Background(background_color).
		Foreground(palette[1])

	return style, nil
}

func (style LoggerStyle) set_primary() LoggerStyle {
	log.PrefixStyle = style.prefix
	log.SeparatorStyle = style.separator

	return style
}

// New logger with the prefix and timestamp.
// It generates the random color style.
func New(prefix string, timestamp bool) (Logger, error) {
	random_style, err := random_style()
	if err != nil {
		return Logger{}, fmt.Errorf("random_style: %w", err)
	}

	logger := log.New()
	logger.SetPrefix(prefix)
	logger.SetReportCaller(false)
	logger.SetReportTimestamp(timestamp)

	new_logger := Logger{
		logger: logger,
		style:  random_style,
	}

	return new_logger, nil
}

// Fatal calls the Error, then os.Exit()
func Fatal(title string, keyval ...interface{}) {
	log.Fatal(title, keyval...)
}

// Info prints the information
func (logger *Logger) Info(title string, keyval ...interface{}) {
	logger.style.set_primary()
	logger.logger.Info(title, keyval...)
}

// Fatal prints the error message and then calls the os.Exit()
func (logger *Logger) Fatal(title string, keyval ...interface{}) {
	logger.style.set_primary()
	logger.logger.Fatal(title, keyval...)
}

// Warn prints the warning message
func (logger *Logger) Warn(title string, keyval ...interface{}) {
	logger.style.set_primary()
	logger.logger.Warn(title, keyval...)
}

// Error prints the error message
func (logger *Logger) Error(title string, keyval ...interface{}) {
	logger.style.set_primary()
	logger.logger.Error(title, keyval...)
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
func (parent *Logger) Child(prefix string, timestamp bool) (Logger, error) {
	child := parent.logger.With()
	child.SetReportTimestamp(WITH_TIMESTAMP)

	child.SetPrefix(parent.logger.GetPrefix() + "/" + prefix)

	return Logger{
		logger: child,
		style:  parent.style,
	}, nil
}

// Same as Child but without printing timestamps
func (parent *Logger) ChildWithoutReport(prefix string) (Logger, error) {
	child, err := parent.Child(prefix, WITH_TIMESTAMP)
	if err != nil {
		return Logger{}, fmt.Errorf("parent.Child: %w", err)
	}

	return child, nil
}

// Same as Child but prints the timestamps
func (parent *Logger) ChildWithTimestamp(prefix string) (Logger, error) {
	child, err := parent.Child(prefix, WITH_TIMESTAMP)
	if err != nil {
		return Logger{}, fmt.Errorf("parent.Child: %w", err)
	}

	return child, nil
}

// Flag to include the timestamps
const WITH_TIMESTAMP = true

// Flag to exclude the timestamps
const WITHOUT_TIMESTAMP = false
