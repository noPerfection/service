package log

import (
	"github.com/charmbracelet/log"
)

func New() log.Logger {
	return log.New()
}

func Child(parent log.Logger, prefix string) log.Logger {
	child := parent.With()
	child.SetPrefix(parent.GetPrefix() + " > " + prefix)

	return child
}
