package proxy

import (
	"github.com/ahmetson/service-lib/communication/command"
	"github.com/ahmetson/service-lib/communication/message"
	"github.com/ahmetson/service-lib/controller"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/service-lib/remote"
)

var anyHandler command.HandleFunc = func(request message.Request, _ log.Logger, extensions remote.Clients) message.Reply {
	proxyClient := remote.GetClient(extensions, ControllerName)
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
	sourceController.RegisterCommand(command.Any, anyHandler)
}
