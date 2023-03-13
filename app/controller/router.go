package controller

import (
	"fmt"

	"github.com/charmbracelet/log"

	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/common/data_type/key_value"

	app_log "github.com/blocklords/sds/app/log"

	zmq "github.com/pebbe/zmq4"
)

type Dealer struct {
	service *service.Service
	socket  *zmq.Socket
}

type Router struct {
	service *service.Service
	dealers []*Dealer
	// command name to the dealer
	routes key_value.KeyValue
	logger log.Logger
}

// Returns the initiated Router whith the service parameters
func NewRouter(parent_log log.Logger, service *service.Service) Router {
	logger := app_log.Child(parent_log, "router")

	routes := key_value.Empty()
	dealers := make([]*Dealer, 0)

	return Router{logger: logger, service: service, routes: routes, dealers: dealers}
}

// Whether the dealer for the service is added or not
func (r *Router) service_registered(service *service.Service) bool {
	for _, dealer := range r.dealers {
		if dealer.service.Url() == service.Url() {
			return true
		}
	}

	return false
}

// Whether the command registered or not
func (r *Router) command_registered(name string) bool {
	_, err := r.routes.GetUint64(name)
	return err == nil
}

func (r *Router) add_service(service *service.Service) uint64 {
	dealer := Dealer{service: service, socket: nil}
	r.dealers = append(r.dealers, &dealer)

	return uint64(len(r.dealers) - 1)
}

// Registers the route from command to dealer.
// SDS Core can have unique command handlers.
func (router *Router) AddDealer(service *service.Service, commands []string) error {
	if router.service_registered(service) {
		return fmt.Errorf("duplicate service url '%s'", service.Url())
	}
	index := router.add_service(service)

	for _, name := range commands {
		if router.command_registered(name) {
			return fmt.Errorf("duplicate command name '%s'", name)
		}

		router.routes.Set(name, index)
	}
	return nil
}

func (router *Router) add_socket(index uint64) error {
	socket, err := zmq.NewSocket(zmq.DEALER)
	if err != nil {
		return fmt.Errorf("error creating socket: %w", err)
	}
	err = socket.Connect(router.dealers[index].service.Url())
	if err != nil {
		return fmt.Errorf("setup of dealer socket: %w", err)
	}

	router.dealers[index].socket = socket

	return nil
}

// Returns the route to the dealer based on the command name
func (router *Router) get_route(name string) *Dealer {
	for command_name, index := range router.routes {
		if command_name == name {
			return router.dealers[index.(uint64)]
		}
	}

	return nil
}

// Runs the router that redirects the incoming requests to the services.
func (router *Router) Run() {
	router.logger.Info("setup the dealer sockets")
	//  Initialize poll set
	poller := zmq.NewPoller()

	// let's set the socket
	for index := range router.dealers {
		err := router.add_socket(uint64(index))
		if err != nil {
			router.logger.Fatal("add_socket", "dealer #", index, "url", router.dealers[index].service.Url())
		}
		poller.Add(router.dealers[index].socket, zmq.POLLIN)
	}
	router.logger.Info("dealers set up successfully")
	router.logger.Info("setup router")

	frontend, _ := zmq.NewSocket(zmq.ROUTER)
	defer frontend.Close()
	err := frontend.Bind(router.service.Url())
	if err != nil {
		router.logger.Fatal("zmq new router", "error", err)
	}

	poller.Add(frontend, zmq.POLLIN)

	//  Switch messages between sockets
	for {
		sockets, _ := poller.Poll(-1)
		for _, socket := range sockets {
			zmq_socket := socket.Socket
			// redirect to the dealer
			if zmq_socket == frontend {
				msgs, err := zmq_socket.RecvMessage(0)
				if err != nil {
					fail := message.Fail("frontend receive message error " + err.Error())
					fail_string, _ := fail.ToString()

					_, err := frontend.Send(msgs[0], zmq.SNDMORE)
					if err != nil {
						router.logger.Fatal("failed to send back id to frontend", "msgs", "error", fail_string)
					}
					_, err = frontend.Send(msgs[1], zmq.SNDMORE)
					if err != nil {
						router.logger.Fatal("failed to send back delimiter to frontend", "error", fail_string)
					}
					_, err = frontend.Send(fail_string, 0)
					if err != nil {
						router.logger.Fatal("failed to send back error to frontend", "error", fail_string)
					}
					continue
				}

				if len(msgs) < 4 {
					fail := message.Fail("invalid message. it should have atleast 4 parts")
					fail_string, _ := fail.ToString()

					_, err := frontend.Send(msgs[0], zmq.SNDMORE)
					if err != nil {
						router.logger.Fatal("failed to send back id to frontend", "msgs", msgs, "fail", fail_string)
					}
					_, err = frontend.Send(msgs[1], zmq.SNDMORE)
					if err != nil {
						router.logger.Fatal("failed to send back delimiter to frontend", "msgs", msgs, "fail", fail_string)
					}
					_, err = frontend.Send(fail_string, 0)
					if err != nil {
						router.logger.Fatal("failed to send back error to frontend", "msgs", msgs, "fail", fail_string)
					}
					continue
				}
				dealer := router.get_route(msgs[2])
				if dealer == nil {
					fail := message.Fail("no route to the socket for command " + msgs[2])
					fail_string, _ := fail.ToString()

					_, err := frontend.Send(msgs[0], zmq.SNDMORE)
					if err != nil {
						router.logger.Fatal("failed to send back id to frontend", "msgs", msgs[2:], "fail", fail_string)
					}
					_, err = frontend.Send(msgs[1], zmq.SNDMORE)
					if err != nil {
						router.logger.Fatal("failed to send back delimiter to frontend", "msgs", msgs[2:], "fail", fail_string)
					}
					_, err = frontend.Send(fail_string, 0)
					if err != nil {
						router.logger.Fatal("failed to send back error to frontend", "msgs", msgs[2:], "fail", fail_string)
					}
					continue
				}

				// router id
				dealer.socket.SendMessage(msgs)
			} else {
				for {
					msg, _ := zmq_socket.Recv(0)
					if more, _ := zmq_socket.GetRcvmore(); more {
						frontend.Send(msg, zmq.SNDMORE)
					} else {
						frontend.Send(msg, 0)
						break
					}
				}
			}
		}
	}
}
