package handler

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/common-lib/message"
	"github.com/ahmetson/log-lib"
	"github.com/ahmetson/os-lib/net"
	"github.com/ahmetson/os-lib/process"
	"github.com/ahmetson/service-lib/communication/command"
	"github.com/ahmetson/service-lib/config/service"
	zmq "github.com/pebbe/zmq4"
)

func newController(logger *log.Logger) *Controller {
	return &Controller{
		logger:             logger,
		controllerType:     service.UnknownType,
		routes:             command.NewRoutes(),
		requiredExtensions: make([]string, 0),
		extensionConfigs:   key_value.Empty(),
		extensions:         key_value.Empty(),
	}
}

// SyncReplier creates a new synchronous Reply handler.
func SyncReplier(parent *log.Logger) (*Controller, error) {
	logger := parent.Child("handler", "type", service.SyncReplierType)

	instance := newController(logger)
	instance.controllerType = service.SyncReplierType

	return instance, nil
}

func (c *Controller) prepare() error {
	if err := c.extensionsAdded(); err != nil {
		return fmt.Errorf("extensionsAdded: %w", err)
	}
	if err := c.initExtensionClients(); err != nil {
		return fmt.Errorf("initExtensionClients: %w", err)
	}
	if c.config == nil || len(c.config.Instances) == 0 {
		return fmt.Errorf("handler doesn't have the config or instances are missing")
	}

	return nil
}

func Bind(sock *zmq.Socket, url string, port uint64) error {
	if err := sock.Bind(url); err != nil {
		if port > 0 {
			// for now, the host name is hardcoded. later we need to get it from the orchestra
			if net.IsPortUsed("localhost", port) {
				pid, err := process.PortToPid(port)
				if err != nil {
					err = fmt.Errorf("config.PortToPid(%d): %w", port, err)
				} else {
					currentPid := process.CurrentPid()
					if currentPid == pid {
						err = fmt.Errorf("another dependency is using it within this orchestra")
					} else {
						err = fmt.Errorf("operating system uses it for another service. pid=%d", pid)
					}
				}
			} else {
				err = fmt.Errorf(`handler.socket.Bind("tcp://*:%d)": %w`, port, err)
			}
			return err
		} else {
			return fmt.Errorf(`handler.socket.bind("inproc://%s"): %w`, url, err)
		}
	}

	return nil
}

func (c *Controller) processMessage(msgRaw []string, metadata map[string]string) (message.Reply, error) {
	// All request types derive from the basic request.
	// We first attempt to parse basic request from the raw message
	request, err := message.ParseRequest(msgRaw)
	if err != nil {
		newErr := fmt.Errorf("message.ParseRequest: %w", err)

		return message.Reply{}, newErr
	}
	pubKey, ok := metadata["pub_key"]
	if ok {
		request.SetPublicKey(pubKey)
	}

	// Add the trace
	if request.IsFirst() {
		request.SetUuid()
	}
	request.AddRequestStack(c.serviceUrl, c.config.Category, c.config.Instances[0].Id)

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
	if err = reply.SetStack(c.serviceUrl, c.config.Category, c.config.Instances[0].Id); err != nil {
		c.logger.Warn("failed to update the reply stack", "error", err)
	}

	return reply, nil
}

func (c *Controller) Run() error {
	var err error
	if err := c.prepare(); err != nil {
		return fmt.Errorf("handler.prepare: %w", err)
	}

	// Socket to talk to clients
	c.socket, err = zmq.NewSocket(zmq.REP)
	if err != nil {
		return fmt.Errorf("zmq.NewSocket: %w", err)
	}

	// if secure and not inproc
	// then we add the domain name of handler to the security layer,
	//
	// then any pass-listing users will be sent there.
	c.logger.Warn("todo", "todo 1", "make sure that all ports are different")

	url := Url(c.config.Instances[0].ControllerCategory, c.config.Instances[0].Port)
	c.logger.Warn("config.Instances[0] is hardcoded. Create multiple instances", "url", url, "name", c.config.Instances[0].ControllerCategory)

	if err := Bind(c.socket, url, c.config.Instances[0].Port); err != nil {
		return fmt.Errorf(`bind("%s"): %w`, c.config.Instances[0].ControllerCategory, err)
	}

	poller := zmq.NewPoller()
	poller.Add(c.socket, zmq.POLLIN)

	for {
		sockets, err := poller.Poll(-1)
		if err != nil {
			newErr := fmt.Errorf("poller.Poll(%s): %w", c.config.Category, err)
			return newErr
		}

		if len(sockets) > 0 {
			msgRaw, metadata, err := c.socket.RecvMessageWithMetadata(0, requiredMetadata()...)
			if err != nil {
				newErr := fmt.Errorf("socket.recvMessageWithMetadata: %w", err)
				if err := c.replyError(c.socket, newErr); err != nil {
					return err
				}
				return newErr
			}

			reply, err := c.processMessage(msgRaw, metadata)
			if err != nil {
				if err := c.replyError(c.socket, err); err != nil {
					return fmt.Errorf("replyError: %w", err)
				}
			} else {
				if err := c.reply(c.socket, reply); err != nil {
					return fmt.Errorf("reply: %w: ", err)
				}
			}
		}
	}
}
