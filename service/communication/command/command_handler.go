// Package command defines the request commands that SDS Services will accept.
// Besides the commands, this package also defines the HandleFunc.
//
// The HandleFunc is the function that executes the command and then returns the result
// to the caller.
package command

import (
	"github.com/blocklords/sds/service/communication/message"
	"github.com/blocklords/sds/service/log"
)

// HandleFunc is the function type that manipulates the commands.
// It accepts atleast message.Request and log.Logger then returns message.Reply.
//
// Optionally the controller can pass the shared states in the additional parameters.
// The most use case for optional parameter is to pass the link to the Database.
type HandleFunc = func(message.Request, log.Logger, ...interface{}) message.Reply

// Binding of Command to the Command Handler.
type Handlers map[CommandName]HandleFunc

// EmptyHandlers returns an empty handler
func EmptyHandlers() Handlers {
	return Handlers{}
}

// Exist returns true if the handler function exists for the command.
func (c Handlers) Exist(command CommandName) bool {
	_, ok := c[command]
	return ok
}

// Adds the Binding of command to handler in the handlers
func (c Handlers) Add(command CommandName, handler HandleFunc) Handlers {
	c[command] = handler
	return c
}

// CommandNames is the list of command names without handlers
func (c Handlers) CommandNames() []string {
	commands := make([]string, len(c))

	i := 0
	for name := range c {
		commands[i] = name.String()
		i++
	}

	return commands
}
