package controller

import (
	"fmt"

	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"

	"github.com/blocklords/sds/app/log"

	zmq "github.com/pebbe/zmq4"
)

type Dealer struct {
	service *service.Service
	socket  *zmq.Socket
}

type Router struct {
	service *service.Service
	dealers []*Dealer
	logger  log.Logger
}

// Returns the initiated Router whith the service parameters
func NewRouter(parent_log log.Logger, service *service.Service) Router {
	logger := parent_log.ChildWithoutReport("router")

	dealers := make([]*Dealer, 0)

	return Router{logger: logger, service: service, dealers: dealers}
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

func (r *Router) add_service(service *service.Service) {
	dealer := Dealer{service: service, socket: nil}
	r.dealers = append(r.dealers, &dealer)
}

// Registers the route from command to dealer.
// SDS Core can have unique command handlers.
func (router *Router) AddDealers(services ...*service.Service) error {
	for _, service := range services {
		if router.service_registered(service) {
			return fmt.Errorf("duplicate service url '%s'", service.Url())
		}
		router.add_service(service)
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
func (router *Router) get_dealer(service string) *Dealer {
	for _, dealer := range router.dealers {
		if dealer.service.Name == service {
			return dealer
		}
	}

	return nil
}

// Runs the router (asynchronous zmq.REP) along with dealers (asynchronous zmq.REQ).
// The dealers are connected to the zmq.REP controllers.
//
// The router will redirect the message to the zmq.REP controllers using dealers.
//
// Note!!! If the request to this router comes from zmq.REQ client, then client
// should set the identity using zmq.SetIdentity().
//
// Format of the incoming message:
//
//		0 - bytes request id
//		1 - ""; empty delimiter
//		2 - string (gosds/app/service.ServiceType) service name as a tag of dealer.
//	     to identify which dealer to use
//		3 - gosds/app/remote/message.Request the message that is redirected to the zmq.REP controller
//
// The request id is used to identify the client. Once the dealer gets the reply from zmq.REP controller
// the router will return it back to the client by request id.
//
// Example:
//
//	// route the msg[3] to the SDS Static
//	msg := [0: "uid-123", 1: "", 2: "static", 3: "{`command`: `smartcontract_get`, `parameters`: {}}"]
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

	router.logger.Info("The router waits for client requests", "service", router.service.Name, "url", router.service.Url())

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
				dealer := router.get_dealer(msgs[2])
				if dealer == nil {
					fail := message.Fail("no dealer registered for " + msgs[2])
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

				// send the id
				dealer.socket.Send(msgs[0], zmq.SNDMORE)
				// send the delimiter
				dealer.socket.Send(msgs[1], zmq.SNDMORE)
				// skip the command name
				// we skip the router name,
				// sending the message.Request part
				last_index := len(msgs) - 1
				for i := 3; i <= last_index; i++ {
					if i == last_index {
						dealer.socket.Send(msgs[i], 0)
					} else {
						dealer.socket.Send(msgs[i], zmq.SNDMORE)
					}
				}
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
