// Package command defines the request commands that SDS Service will accept.
// Besides the commands, this package also defines the HandleFunc.
//
// The HandleFunc is the function that executes the command and then returns the result
// to the caller.
package command

import (
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/communication/message"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/service-lib/remote"
)

// HandleFunc is the function type that manipulates the commands.
// It accepts at least message.Request and log.Logger then returns message.Reply.
//
// Optionally the controller can pass the shared states in the additional parameters.
// The most use case for optional parameter is to pass the link to the Database.
type HandleFunc = func(message.Request, *log.Logger, ...*remote.ClientSocket) message.Reply

// Routes Binding of Command to the Command Handler.
type Routes = key_value.List

// NewRoutes returns an empty routes
func NewRoutes() *Routes {
	return key_value.NewList()
}

// Commands returns the commands from the routes
func Commands(routes *Routes) []string {
	commands := make([]string, routes.Len())

	list := routes.List()

	i := 0
	for name := range list {
		commands[i] = name.(string)
		i++
	}

	return commands
}
