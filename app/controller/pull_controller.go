/*
Controller package is the interface of the module.
It acts as the input receiver for other services or for external users.
*/
package controller

import (
	"fmt"

	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/service"

	zmq "github.com/pebbe/zmq4"
)

// Creates a synchrounous Reply controller.
func NewPull(s *service.Service, logger log.Logger) (*Controller, error) {
	if !s.IsThis() && !s.IsInproc() {
		return nil, fmt.Errorf("service should be limited to service.THIS or inproc type")
	}
	controller_logger, err := logger.ChildWithTimestamp("pull_" + s.Name)
	if err != nil {
		return nil, fmt.Errorf("error creating child logger: %w", err)
	}

	// Socket to talk to clients
	socket, err := zmq.NewSocket(zmq.PULL)
	if err != nil {
		return nil, fmt.Errorf("zmq.NewSocket: %w", err)
	}

	return &Controller{
		socket:      socket,
		service:     s,
		logger:      controller_logger,
		socket_type: zmq.PULL,
	}, nil
}
