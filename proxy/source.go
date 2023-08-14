package proxy

import (
	"fmt"
	"github.com/ahmetson/common-lib/message"
	"github.com/ahmetson/log-lib"
	"github.com/ahmetson/service-lib/client"
	"github.com/ahmetson/service-lib/communication/command"
	"github.com/ahmetson/service-lib/server"
)

var anyHandler = func(request message.Request, _ *log.Logger, extensions ...*client.ClientSocket) message.Reply {
	proxyClient := client.FindClient(extensions, ControllerName)
	replyParameters, err := proxyClient.RequestRemoteService(&request)
	if err != nil {
		return request.Fail(err.Error())
	}

	reply := message.Reply{
		Status:     message.OK,
		Message:    "",
		Parameters: replyParameters,
	}
	return reply
}

// SourceHandler makes the given server as the source of the proxy.
// It means, it will add command.Any to call the proxy.
func SourceHandler(sourceController server.Interface) error {
	route := command.NewRoute(command.Any, anyHandler, ControllerName)

	if err := sourceController.AddRoute(route); err != nil {
		return fmt.Errorf("failed to add any route into the server: %w", err)
	}
	return nil
}
