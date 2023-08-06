package command

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/communication/message"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/service-lib/remote"
)

// Route is the command, handler of the command
// and the extensions that this command depends on.
type Route struct {
	Command    string
	Extensions []string
	handler    HandleFunc
}

// Any command name
const Any string = "*"

// NewRoute returns a new command handler. It's used by the controllers.
func NewRoute(command string, handler HandleFunc, extensions ...string) *Route {
	return &Route{
		Command:    command,
		Extensions: extensions,
		handler:    handler,
	}
}

// AddHandler if the handler already exists, then it will throw an error
func (route *Route) AddHandler(handler HandleFunc) error {
	if route.handler == nil {
		route.handler = handler
		return nil
	}

	return fmt.Errorf("handler exists in %s route", route.Command)
}

// FilterExtensionClients returns the list of the clients specific for this command
func (route *Route) filterExtensionClients(clients remote.Clients) []*remote.ClientSocket {
	routeClients := make([]*remote.ClientSocket, len(route.Extensions))

	added := 0
	for extensionName := range clients {
		for i := 0; i < len(route.Extensions); i++ {
			if route.Extensions[i] == extensionName {
				routeClients[added] = clients[extensionName].(*remote.ClientSocket)
				added++
			}
		}
	}

	return routeClients
}

func (route *Route) Handle(request message.Request, logger *log.Logger, allExtensions remote.Clients) message.Reply {
	extensions := route.filterExtensionClients(allExtensions)
	return route.handler(request, logger, extensions...)
}

// Reply creates a successful message.Reply with the given reply parameters.
func Reply(reply interface{}) (message.Reply, error) {
	replyParameters, err := key_value.NewFromInterface(reply)
	if err != nil {
		return message.Reply{}, fmt.Errorf("failed to encode reply: %w", err)
	}

	return message.Reply{
		Status:     message.OK,
		Message:    "",
		Parameters: replyParameters,
	}, nil
}
