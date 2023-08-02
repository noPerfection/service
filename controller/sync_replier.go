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
func SyncReplier(parent *log.Logger) (*Controller, error) {
	logger := parent.Child("controller", "type", configuration.ReplierType)

	return &Controller{
		logger:             logger,
		controllerType:     configuration.ReplierType,
		routes:             command.NewRoutes(),
		requiredExtensions: make([]string, 0),
		extensionConfigs:   key_value.Empty(),
		extensions:         key_value.Empty(),
	}, nil
}

func (c *Controller) Run() error {
	var err error
	if err := c.extensionsAdded(); err != nil {
		return fmt.Errorf("extensionsAdded: %w", err)
	}
	if err := c.initExtensionClients(); err != nil {
		return fmt.Errorf("initExtensionClients: %w", err)
	}
	if c.config == nil || len(c.config.Instances) == 0 {
		return fmt.Errorf("controller doesn't have the configuration or instances are missing")
	}

	// Socket to talk to clients
	c.socket, err = zmq.NewSocket(zmq.REP)
	if err != nil {
		return fmt.Errorf("zmq.NewSocket: %w", err)
	}

	// if secure and not inproc
	// then we add the domain name of controller to the security layer
	//
	// then any whitelisting users will be sent there.
	c.logger.Warn("todo", "todo 1", "make sure that all ports are different")

	url := Url(c.config.Instances[0].Name, c.config.Instances[0].Port)
	c.logger.Warn("config.Instances[0] is hardcoded. Create multiple instances", "url", url, "name", c.config.Instances[0].Name)

	if err := c.socket.Bind(url); err != nil {
		port := c.config.Instances[0].Port
		if port > 0 {
			// for now, the host name is hardcoded. later, we need to get it from the context
			if configuration.IsPortUsed("localhost", port) {
				c.logger.Info("Port is used, let's find out who uses it", "port", port)
				pid, err := configuration.PortToPid(port)
				c.logger.Info("pid by port is returned", "pid", pid, "error", err)
				if err != nil {
					c.logger.Error("failed to get the pid by port")
					err = fmt.Errorf("configuration.PortToPid(%d): %w", port, err)
				} else {
					c.logger.Info("comparing is it within this process or by another process")
					currentPid := configuration.CurrentPid()
					c.logger.Info("read runtime", "current pid", currentPid)
					if currentPid == pid {
						err = fmt.Errorf("another dependency is using it within this context")
					} else {
						err = fmt.Errorf("operating system uses it for another service. pid=%d", pid)
					}
				}
			} else {
				err = fmt.Errorf(`controller("%s").socket.Bind("tcp://*:%d)": %w`, c.config.Instances[0].Name, &c.config.Instances[0].Port, err)
			}
			return err
		} else {
			return fmt.Errorf(`controller("%s").socket.bind("inproc://%s"): %w`, c.config.Instances[0].Name, url, err)
		}
	}

	poller := zmq.NewPoller()
	poller.Add(c.socket, zmq.POLLIN)

	for {
		sockets, err := poller.Poll(-1)
		if err != nil {
			newErr := fmt.Errorf("poller.Poll(%s): %w", c.config.Name, err)
			return newErr
		}

		if len(sockets) > 0 {
			msgRaw, metadata, err := c.socket.RecvMessageWithMetadata(0, requiredMetadata()...)
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
				reply = request.Fail("route get " + request.Command + " failed: " + err.Error())
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
}
