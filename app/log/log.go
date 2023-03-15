package log

import (
	"github.com/charmbracelet/log"
)

type Logger struct {
	logger log.Logger
}

// Create a new logger with the report caller, and timestamp
func New(prefix string, report_caller bool, timestamp bool) Logger {
	logger := log.New()
	logger.SetPrefix(prefix)
	logger.SetReportCaller(report_caller)
	logger.SetReportTimestamp(timestamp)

	return Logger{
		logger: logger,
	}
}

func (logger Logger) Info(title string, keyval ...interface{}) {
	logger.logger.Info(title, keyval...)
}

func (logger Logger) Fatal(title string, keyval ...interface{}) {
	logger.logger.Fatal(title, keyval...)
}

func (logger Logger) Warn(title string, keyval ...interface{}) {
	logger.logger.Warn(title, keyval...)
}

func (logger Logger) Error(title string, keyval ...interface{}) {
	logger.logger.Error(title, keyval...)
}

// Create a new child from the parent
func (parent Logger) Child(prefix string, report_caller bool, timestamp bool) Logger {
	child := parent.logger.With()
	child.SetReportCaller(report_caller)
	child.SetReportTimestamp(timestamp)

	child.SetPrefix(parent.logger.GetPrefix() + "::" + prefix)

	return Logger{
		logger: child,
	}
}

func (parent Logger) ChildWithoutReport(prefix string) Logger {
	return parent.Child(prefix, WITHOUT_REPORT_CALLER, WITHOUT_TIMESTAMP)
}

func (parent Logger) ChildWithTimestamp(prefix string) Logger {
	return parent.Child(prefix, WITHOUT_REPORT_CALLER, WITH_TIMESTAMP)
}

const WITH_REPORT_CALLER = true
const WITHOUT_REPORT_CALLER = false
const WITH_TIMESTAMP = true
const WITHOUT_TIMESTAMP = false
