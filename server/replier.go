package server

// Asynchronous replier

import (
	"fmt"
	"github.com/ahmetson/common-lib/message"
	"github.com/ahmetson/log-lib"
	"github.com/ahmetson/service-lib/config/service"
	zmq "github.com/pebbe/zmq4"
	"runtime"
)

type base = Controller

// AsyncController is the socket wrapper for the service.
type AsyncController struct {
	*base
	manager    *zmq.Socket
	reactor    *zmq.Reactor
	workers    []string
	maxWorkers int
}

const (
	WorkerReady = "\001" // Signals worker is ready
)

// Workers are calling command handlers defined in command.HandleFunc
func (c *AsyncController) worker() {
	socket, _ := zmq.NewSocket(zmq.REQ)
	err := socket.Connect(c.managerUrl())
	if err != nil {
		c.logger.Fatal("finished working")
	}
	_, err = socket.SendMessage(WorkerReady)
	if err != nil {
		c.logger.Fatal("send message")
	}

	poller := zmq.NewPoller()
	poller.Add(socket, zmq.POLLIN)

	for {
		sockets, err := poller.Poll(-1)
		if err != nil {
			newErr := fmt.Errorf("poller.Poll(%s): %w", c.config.Category, err)
			c.logger.Error("worker polling:", "error", newErr)
		}

		if len(sockets) > 0 {
			msgRaw, metadata, err := socket.RecvMessageWithMetadata(0, requiredMetadata()...)
			if err != nil {
				newErr := fmt.Errorf("socket.recvMessageWithMetadata: %w", err)
				req := message.Request{}
				reply := req.Fail(newErr.Error())
				replyStr, _ := reply.String()
				if _, err := socket.SendMessage(msgRaw[0], msgRaw[1], replyStr); err != nil {
					c.logger.Error("error occurred", "error", fmt.Errorf("recv error replying error %w"+err.Error()))
					continue
				}
			}

			reply, err := c.processMessage(msgRaw[2:], metadata)
			if err != nil {
				newErr := fmt.Errorf("socket.processMessage: %w", err)
				req := message.Request{}
				reply := req.Fail(newErr.Error())
				replyStr, _ := reply.String()
				if _, err := socket.SendMessage(msgRaw[0], msgRaw[1], replyStr); err != nil {
					c.logger.Error("error occurred", "error", fmt.Errorf("recv error replying error %w"+err.Error()))
				}
			} else {
				replyStr, _ := reply.String()
				if _, err := socket.SendMessage(msgRaw[0], msgRaw[1], replyStr); err != nil {
					c.logger.Error("error occurred", "error", fmt.Errorf("recv error replying error %w"+err.Error()))
				}
			}
		}
	}
}

// handleFrontend is an event invoked by the zmq4.Reactor whenever a new client request happens.
//
// This function will forward the messages to the backend.
// Since backend is calling the workers which means the worker will be busy, this function removes the worker from the queue.
// Since the queue is removed, it will remove the frontend from the reactor.
// Frontend will still receive the messages, however they will be queued until frontend will not be added to the reactor.
func (c *AsyncController) handleFrontend() error {
	msg, err := c.socket.RecvMessage(0)
	if err != nil {
		return err
	}

	_, err = c.manager.SendMessage(c.workers[0], "", msg)
	if err != nil {
		return fmt.Errorf("manager.SendMessage: %w", err)
	}
	c.workers = c.workers[1:]

	// stop accepting messages from the frontend.
	// it means, we will not call this function for any incoming request.
	// the frontend will queue the messages in the background.
	if len(c.workers) == 0 {
		c.reactor.RemoveSocket(c.socket)
	}

	return nil
}

// handleBackend is an event invoked by the zmq4.Reactor whenever a worker request happens.
//
// whenever a backend receives the message, it receives from the worker.
// that means the worker is free.
// if a worker is free, then add to the zmq4.Reactor our frontend.
// reactor will consume queued messages from the clients
func (c *AsyncController) handleBackend() error {
	msg, err := c.manager.RecvMessage(0)
	if err != nil {
		return nil
	}

	identity, msg := unwrap(msg)
	c.workers = append(c.workers, identity)

	if len(c.workers) == 1 {
		c.reactor.AddSocket(c.socket, zmq.POLLIN, func(e zmq.State) error { return c.handleFrontend() })
	}

	if msg[0] != WorkerReady {
		if _, err = c.socket.SendMessage(msg); err != nil {
			return fmt.Errorf("socket.SendMessage: %w", err)
		}
	}

	return nil
}

// Replier creates an asynchronous replying server.
func Replier(parent *log.Logger) (*AsyncController, error) {
	logger := parent.Child("async-server", "type", service.ReplierType)

	maxWorkers := runtime.NumCPU()

	base := newController(logger)
	base.controllerType = service.ReplierType

	instance := &AsyncController{
		base:       base,
		manager:    nil,
		reactor:    nil,
		workers:    make([]string, maxWorkers),
		maxWorkers: maxWorkers,
	}

	return instance, nil

}

// call it only after adding a config.
// returns an inproc url
//
// the name of the server should not contain a space or special character
func (c *AsyncController) managerUrl() string {
	name := "async_manager_" + c.config.Instances[0].ControllerCategory

	return Url(name, 0)
}

func (c *AsyncController) Run() error {
	if err := c.base.prepare(); err != nil {
		return fmt.Errorf("base.prepare: %w", err)
	}

	c.socket, _ = zmq.NewSocket(zmq.ROUTER)
	c.manager, _ = zmq.NewSocket(zmq.ROUTER)

	url := Url(c.config.Instances[0].ControllerCategory, c.config.Instances[0].Port)
	if err := Bind(c.socket, url, c.config.Instances[0].Port); err != nil {
		return fmt.Errorf("bind('%s'): %w", c.config.Instances[0].ControllerCategory, err)
	}

	url = c.managerUrl()
	if err := Bind(c.manager, url, 0); err != nil {
		return fmt.Errorf("bind('%s'): %w", c.config.Instances[0].ControllerCategory, err)
	}

	for i := 0; i < c.maxWorkers; i++ {
		go c.worker()
	}

	c.workers = make([]string, 0, c.maxWorkers)

	c.reactor = zmq.NewReactor()
	c.reactor.AddSocket(c.manager, zmq.POLLIN,
		func(e zmq.State) error { return c.handleBackend() })

	if err := c.reactor.Run(-1); err != nil {
		return fmt.Errorf("react.Run: %w", err)
	}
	return nil
}

func unwrap(msg []string) (head string, tail []string) {
	head = msg[0]
	if len(msg) > 1 && msg[1] == "" {
		tail = msg[2:]
	} else {
		tail = msg[1:]
	}

	return
}
