package controller

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/communication/command"
	"github.com/ahmetson/service-lib/communication/message"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/log"

	zmq "github.com/pebbe/zmq4"
)

// SyncReplier creates a new synchronous Reply controller.
func SyncReplier(logger *log.Logger) (*Controller, error) {
	controllerLogger := logger.Child("controller", "type", configuration.ReplierType)

	// Socket to talk to clients
	socket, err := zmq.NewSocket(zmq.REP)
	if err != nil {
		return nil, fmt.Errorf("zmq.NewSocket: %w", err)
	}

	return &Controller{
		socket:             socket,
		logger:             controllerLogger,
		controllerType:     configuration.ReplierType,
		routes:             command.NewRoutes(),
		requiredExtensions: make([]string, 0),
		extensionConfigs:   key_value.Empty(),
		extensions:         key_value.Empty(),
	}, nil
}

func Run(c *Controller) error {
	if err := c.extensionsAdded(); err != nil {
		return fmt.Errorf("extensionsAdded: %w", err)
	}
	if err := c.initExtensionClients(); err != nil {
		return fmt.Errorf("initExtensionClients: %w", err)
	}
	if c.config == nil || len(c.config.Instances) == 0 {
		return fmt.Errorf("controller doesn't have the configuration or instances are missing")
	}

	// if secure and not inproc
	// then we add the domain name of controller to the security layer
	//
	// then any whitelisting users will be sent there.
	c.logger.Warn("config.Instances[0] is hardcoded. Create multiple instances")
	c.logger.Warn("todo", "todo 1", "make sure that all ports are different")
	if err := c.socket.Bind(controllerUrl(c.config.Instances[0].Port)); err != nil {
		return fmt.Errorf("socket.bind on tcp protocol for %s at url %d: %w", c.config.Name, c.config.Instances[0].Port, err)
	}

	for {
		msgRaw, metadata, err := c.socket.RecvMessageWithMetadata(0, "pub_key", "Identity")
		if err != nil {
			newErr := fmt.Errorf("socket.recvMessageWithMetadata: %w", err)
			if err := c.replyError(newErr); err != nil {
				return err
			}
			return newErr
		}

		// All request types derive from the basic request.
		// We first attempt to parse basic request from the raw message
		request, err := message.ParseRequest(msgRaw)
		if err != nil {
			newErr := fmt.Errorf("message.ParseRequest: %w", err)
			if err := c.replyError(newErr); err != nil {
				return err
			}
			continue
		}
		pubKey, ok := metadata["pub_key"]
		if ok {
			request.SetPublicKey(pubKey)
		}

		// Add the trace
		if request.IsFirst() {
			request.SetUuid()
		}
		request.AddRequestStack(c.serviceUrl, c.config.Name, c.config.Instances[0].Instance)

		var reply message.Reply
		var routeInterface interface{}

		if c.routes.Exist(request.Command) {
			routeInterface, err = c.routes.Get(request.Command)
		} else if c.routes.Exist(command.Any) {
			routeInterface, err = c.routes.Get(command.Any)
		} else {
			err = fmt.Errorf("handler not found for command: %s", request.Command)
		}

		if err != nil {
			reply = message.Fail("route get " + request.Command + " failed: " + err.Error())
		} else {
			route := routeInterface.(*command.Route)
			// for puller's it returns an error that occurred on the blockchain.
			reply = route.Handle(request, c.logger, c.extensions)
		}

		// update the stack
		if err = reply.SetStack(c.serviceUrl, c.config.Name, c.config.Instances[0].Instance); err != nil {
			c.logger.Warn("failed to update the reply stack", "error", err)
		}

		if err := c.reply(reply); err != nil {
			return err
		}
		if !reply.IsOK() && !c.isReply() {
			c.logger.Warn("handler replied an error", "command", request.Command, "request parameters", request.Parameters, "error message", reply.Message)
		}
	}
}
