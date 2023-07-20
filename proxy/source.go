package proxy

import (
	"github.com/ahmetson/service-lib/communication/command"
	"github.com/ahmetson/service-lib/communication/message"
	"github.com/ahmetson/service-lib/controller"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/service-lib/remote"
)

var anyHandler = func(request message.Request, _ log.Logger, extensions ...*remote.ClientSocket) message.Reply {
	proxyClient := remote.FindClient(extensions, ControllerName)
	replyParameters, err := proxyClient.RequestRemoteService(&request)
	if err != nil {
		return message.Fail(err.Error())
	}

	reply := message.Reply{
		Status:     message.OK,
		Message:    "",
		Parameters: replyParameters,
	}
	return reply
}

// SourceHandler makes the given controller as the source of the proxy.
// It means, it will add command.Any to call the proxy.
func SourceHandler(sourceController controller.Interface) {
	route := command.NewRoute(command.Any, anyHandler, ControllerName)

	sourceController.AddRoute(route)
}
