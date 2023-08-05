package controller

import (
	"fmt"
	"github.com/ahmetson/service-lib/configuration"
	zmq "github.com/pebbe/zmq4"
)

type Instance struct {
	config configuration.Instance
	socket *zmq.Socket
}

func NewInstance(config configuration.Instance) *Instance {
	return &Instance{
		socket: nil,
		config: config,
	}
}

func GetType(controllerType configuration.Type) zmq.Type {
	if controllerType == configuration.SyncReplierType {
		return zmq.REP
	} else if controllerType == configuration.ReplierType {
		return zmq.ROUTER
	}
	return zmq.REP
}

func (instance *Instance) Run(c *Controller) error {
	// Socket to talk to clients
	socket, err := zmq.NewSocket(GetType(c.ControllerType()))
	if err != nil {
		return fmt.Errorf("zmq.NewSocket: %w", err)
	}
	instance.socket = socket

	// if secure and not inproc
	// then we add the domain name of controller to the security layer
	//
	// then any whitelisting users will be sent there.
	c.logger.Warn("todo", "todo 1", "make sure that all ports are different")

	url := Url(c.config.Instances[0].Controller, c.config.Instances[0].Port)
	c.logger.Warn("config.Instances[0] is hardcoded. Create multiple instances", "url", url, "name", c.config.Instances[0].Controller)

	if err := Bind(instance.socket, url, c.config.Instances[0].Port); err != nil {
		return fmt.Errorf(`bind("%s"): %w`, c.config.Instances[0].Controller, err)
	}

	poller := zmq.NewPoller()
	poller.Add(instance.socket, zmq.POLLIN)

	for {
		sockets, err := poller.Poll(-1)
		if err != nil {
			newErr := fmt.Errorf("poller.Poll(%s): %w", c.config.Category, err)
			return newErr
		}

		if len(sockets) > 0 {
			msgRaw, metadata, err := instance.socket.RecvMessageWithMetadata(0, requiredMetadata()...)
			if err != nil {
				newErr := fmt.Errorf("socket.recvMessageWithMetadata: %w", err)
				if err := c.replyError(instance.socket, newErr); err != nil {
					return err
				}
				return newErr
			}

			reply, err := c.processMessage(msgRaw, metadata)
			if err != nil {
				if err := c.replyError(instance.socket, err); err != nil {
					return fmt.Errorf("replyError: %w", err)
				}
			} else {
				if err := c.reply(instance.socket, reply); err != nil {
					return fmt.Errorf("reply: %w: ", err)
				}
			}
		}
	}
}

func (instance *Instance) Close() error {
	if instance.socket == nil {
		return nil
	}

	err := instance.socket.Close()
	if err != nil {
		return fmt.Errorf("controller.socket.Close: %w", err)
	}

	return nil
}
