// Package log defines the logger engine.
// The unique feature is that it can create a child logger derived from the parent logger.
// Each logger defines a unique color style for the message outputs.
//
// Create a child logger for the packages that the service is calling.
package log

import (
	"fmt"
	"image/color"

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
	error_style lipgloss.Style
	info_style  lipgloss.Style
	warn_style  lipgloss.Style
	fatal_style lipgloss.Style

	timestamp lipgloss.Style
	prefix    lipgloss.Style
	message   lipgloss.Style
	key       lipgloss.Style
	value     lipgloss.Style
	separator lipgloss.Style
}

func random_style() (LoggerStyle, error) {
	raw_palette, err := gamut.Generate(10, gamut.PastelGenerator{})
	if err != nil {
		return LoggerStyle{}, fmt.Errorf("color.Generate: %w", err)
	}
	palette := make([]lipgloss.Color, len(raw_palette))
	for i, raw_palette := range raw_palette {
		palette[i] = lipgloss.Color(to_hex(raw_palette))
	}

	raw_background_color := gamut.Contrast(raw_palette[0])
	background_color := lipgloss.Color(to_hex(raw_background_color))

	style := LoggerStyle{}

	style.error_style = lipgloss.NewStyle().
		SetString("ERROR").
		Bold(true).
		MaxWidth(4).
		Background(background_color).
		Foreground(palette[0])

	style.info_style = lipgloss.NewStyle().
		SetString("INFO").
		Bold(true).
		MaxWidth(4).
		Background(background_color).
		Foreground(palette[1])

	style.fatal_style = lipgloss.NewStyle().
		SetString("FATAL").
		Bold(true).
		MaxWidth(4).
		Background(background_color).
		Foreground(palette[2])

	style.warn_style = lipgloss.NewStyle().
		SetString("WARNING").
		Bold(true).
		MaxWidth(4).
		Background(background_color).
		Foreground(palette[3])

	style.timestamp = lipgloss.NewStyle().
		Background(background_color).
		Foreground(palette[4])

	style.prefix = lipgloss.NewStyle().
		Bold(true).Faint(true).
		Background(background_color).
		Foreground(palette[5])

	style.message = lipgloss.NewStyle().
		Background(background_color).
		Foreground(palette[6])

	style.key = lipgloss.NewStyle().
		Background(background_color).
		Foreground(palette[7])

	style.value = lipgloss.NewStyle().
		Background(background_color).
		Foreground(palette[8])

	// SeparatorStyle is the style for separators.
	style.separator = lipgloss.NewStyle().Faint(true).Background(background_color).
		Foreground(palette[9])

	return style, nil
}

func (style LoggerStyle) error() LoggerStyle {
	log.ErrorLevelStyle = style.error_style
	return style.set_primary()
}
func (style LoggerStyle) warn() LoggerStyle {
	log.WarnLevelStyle = style.warn_style
	return style.set_primary()
}

func (style LoggerStyle) info() LoggerStyle {
	log.InfoLevelStyle = style.info_style
	return style.set_primary()
}

func (style LoggerStyle) fatal() LoggerStyle {
	log.FatalLevelStyle = style.fatal_style
	return style.set_primary()
}

func (style LoggerStyle) set_primary() LoggerStyle {
	log.TimestampStyle = style.timestamp
	log.PrefixStyle = style.prefix
	log.MessageStyle = style.message
	log.KeyStyle = style.key
	log.ValueStyle = style.value
	log.SeparatorStyle = style.value

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
	logger.style.info()
	logger.logger.Info(title, keyval...)
}

// Fatal prints the error message and then calls the os.Exit()
func (logger *Logger) Fatal(title string, keyval ...interface{}) {
	logger.style.fatal()
	logger.logger.Fatal(title, keyval...)
}

// Warn prints the warning message
func (logger *Logger) Warn(title string, keyval ...interface{}) {
	logger.style.warn()
	logger.logger.Warn(title, keyval...)
}

// Error prints the error message
func (logger *Logger) Error(title string, keyval ...interface{}) {
	logger.style.error()
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
	random_style, err := random_style()
	if err != nil {
		return Logger{}, fmt.Errorf("random_style: %w", err)
	}

	child := parent.logger.With()
	child.SetReportTimestamp(timestamp)

	child.SetPrefix(parent.logger.GetPrefix() + "::" + prefix)

	return Logger{
		logger: child,
		style:  random_style,
	}, nil
}

// Same as Child but without printing timestamps
func (parent *Logger) ChildWithoutReport(prefix string) (Logger, error) {
	child, err := parent.Child(prefix, WITHOUT_TIMESTAMP)
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

// Hex returns the hex "html" representation of the color, as in #ff0080.
func to_hex(col color.Color) string {
	r, g, b, _ := col.RGBA()
	// Add 0.5 for rounding
	return fmt.Sprintf("#%02x%02x%02x", uint8(float32(r)*255.0+0.5), uint8(float32(g)*255.0+0.5), uint8(float32(b)*255.0+0.5))
}

// Flag to include the timestamps
const WITH_TIMESTAMP = true

// Flag to exclude the timestamps
const WITHOUT_TIMESTAMP = false
