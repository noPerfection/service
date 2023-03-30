package command

import (
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote/message"
)

type HandleFunc = func(message.Request, log.Logger, ...interface{}) message.Reply

// command name => function
type Handlers map[Command]HandleFunc

func EmptyHandlers() Handlers {
	return Handlers{}
}

// Check does command handler exist
func (c Handlers) Exist(command Command) bool {
	_, ok := c[command]
	return ok
}

func (c Handlers) Add(command Command, handler HandleFunc) Handlers {
	c[command] = handler
	return c
}

// Returns the list of command names without handlers
func (c Handlers) CommandNames() []string {
	commands := make([]string, len(c))

	i := 0
	for name := range c {
		commands[i] = name.String()
		i++
	}

	return commands
}
