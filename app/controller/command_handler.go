package controller

import (
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote/message"
)

// command name => function
type CommandHandlers map[string]func(message.Request, log.Logger, ...interface{}) message.Reply

// Check does command handler exist
func (c CommandHandlers) Exist(command string) bool {
	_, ok := c[command]
	return ok
}

// Returns the list of command names without handlers
func (c CommandHandlers) Commands() []string {
	commands := make([]string, len(c))

	i := 0
	for name := range c {
		commands[i] = name
		i++
	}

	return commands
}
