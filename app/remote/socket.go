// This package defines the data types, and methods that interact with a remote SDS service.
//
// The request reply socket follows the Lazy Pirate pattern.
//
// Example using pebbe/zmq4 is here:
// https://github.com/pebbe/zmq4/blob/83013091510dd1275bbf0b9a302533cadc17d392/examples/lpclient.go
//
// The Lazy Pirate pattern is described in the ZMQ guide:
// https://zguide.zeromq.org/docs/chapter4/#Client-Side-Reliability-Lazy-Pirate-Pattern
package remote

import (
	"errors"
	"fmt"

	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/remote/parameter"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/security/credentials"
	zmq "github.com/pebbe/zmq4"
)

// Over the socket the remote call is happening.
// This is the wrapper of zeromq socket. Wrapper enables to create larger network patterns.
type Socket struct {
	// The name of remote SDS service and its URL
	// its used as a clarification
	remote_service     *service.Service
	client_credentials *credentials.Credentials
	poller             *zmq.Poller
	socket             *zmq.Socket
	protocol           string
	inproc_url         string
}

type SDS_Message interface {
	*message.Request

	CommandName() string
	ToString() (string, error)
}

// Initiates the socket with a timeout.
// If the socket is already given, then reconnect() closes it.
// Then creates a new socket.
//
// If no socket is given, then initiates a zmq.REQ socket.
func (socket *Socket) reconnect() error {
	var socket_ctx *zmq.Context
	var socket_type zmq.Type

	if socket.socket != nil {
		ctx, err := socket.socket.Context()
		if err != nil {
			return fmt.Errorf("failed to get context from zmq socket: %w", err)
		} else {
			socket_ctx = ctx
		}

		socket_type, err = socket.socket.GetType()
		if err != nil {
			return fmt.Errorf("failed to get socket type from zmq socket: %w", err)
		}

		err = socket.Close()
		if err != nil {
			return fmt.Errorf("failed to close socket in zmq: %w", err)
		}
		socket.socket = nil
	} else {
		return fmt.Errorf("no socket initiated: %s", "reconnect")
	}

	sock, err := socket_ctx.NewSocket(socket_type)
	if err != nil {
		return fmt.Errorf("failed to create %s socket: %w", socket_type.String(), err)
	} else {
		socket.socket = sock
		err = socket.socket.SetLinger(0)
		if err != nil {
			return fmt.Errorf("failed to set up linger parameter for zmq socket: %w", err)
		}
	}

	if socket.client_credentials != nil {
		socket.client_credentials.SetClientAuthCurve(socket.socket, socket.remote_service.Credentials.PublicKey)
		if err != nil {
			return fmt.Errorf("credentials.SetClientAuthCurve: %w", err)
		}
	}

	if err := socket.socket.Connect(socket.remote_service.Url()); err != nil {
		return fmt.Errorf("socket connect: %w", err)
	}

	socket.poller = zmq.NewPoller()
	socket.poller.Add(socket.socket, zmq.POLLIN)

	return nil
}

func (socket *Socket) inproc_reconnect() error {
	var socket_ctx *zmq.Context
	var socket_type zmq.Type

	if socket.socket != nil {
		ctx, err := socket.socket.Context()
		if err != nil {
			return fmt.Errorf("failed to get context from zmq socket: %w", err)
		} else {
			socket_ctx = ctx
		}

		socket_type, err = socket.socket.GetType()
		if err != nil {
			return fmt.Errorf("failed to get socket type from zmq socket: %w", err)
		}

		err = socket.Close()
		if err != nil {
			return fmt.Errorf("failed to close socket in zmq: %w", err)
		}
		socket.socket = nil
	} else {
		return fmt.Errorf("failed to create zmq context: %s", "inproc_reconnect")
	}

	sock, err := socket_ctx.NewSocket(socket_type)
	if err != nil {
		return fmt.Errorf("failed to create %s socket: %w", socket_type.String(), err)
	} else {
		socket.socket = sock
		err = socket.socket.SetLinger(0)
		if err != nil {
			return fmt.Errorf("failed to set up linger parameter for zmq socket: %w", err)
		}
	}

	if err := socket.socket.Connect(socket.inproc_url); err != nil {
		return fmt.Errorf("error '%s' connect: %w", socket.inproc_url, err)
	}

	socket.poller = zmq.NewPoller()
	socket.poller.Add(socket.socket, zmq.POLLIN)

	return nil
}

// Close the remote connection
func (socket *Socket) Close() error {
	err := socket.socket.Close()
	if err != nil {
		return fmt.Errorf("error closing socket: %w", err)
	}

	return nil
}


// Returns the HOST envrionment parameters of the socket.
//
// Use it if you want to create another socket from this socket.
func (socket *Socket) RemoteEnv() *service.Service {
	return socket.remote_service
}

// Send a command to the remote SDS service.
// Note that it converts the failure reply into an error. Rather than replying reply itself back to user.
// In case of successful request, the function returns reply parameters.
func (socket *Socket) RequestRemoteService(request *message.Request) (key_value.KeyValue, error) {
	request_timeout := parameter.RequestTimeout()

	request_string, err := request.ToString()
	if err != nil {
		return nil, fmt.Errorf("request.ToString: %w", err)
	}

	// we attempt requests for an infinite amount of time.
	for {
		//  We send a request, then we work to get a reply
		if _, err := socket.socket.SendMessage(request_string); err != nil {
			return nil, fmt.Errorf("failed to send the command '%s' to '%s'. socket error: %w", request.Command, socket.remote_service.Name, err)
		}

		//  Poll socket for a reply, with timeout
		sockets, err := socket.poller.Poll(request_timeout)
		if err != nil {
			return nil, fmt.Errorf("failed to to send the command '%s' to '%s'. poll error: %w", request.Command, socket.remote_service.Name, err)
		}

		//  Here we process a server reply and exit our loop if the
		//  reply is valid. If we didn't a reply we close the client
		//  socket and resend the request. We try a number of times
		//  before finally abandoning:

		if len(sockets) > 0 {
			// Wait for reply.
			r, err := socket.socket.RecvMessage(0)
			if err != nil {
				return nil, fmt.Errorf("failed to receive the command '%s' message. socket error: %w", request.Command, err)
			}

			reply, err := message.ParseReply(r)
			if err != nil {
				return nil, fmt.Errorf("failed to parse the command '%s': %w", request.Command, err)
			}

			if !reply.IsOK() {
				return nil, fmt.Errorf("the command '%s' replied with a failure: %s", request.Command, reply.Message)
			}

			return reply.Parameters, nil
		} else {
			fmt.Println("timeout", socket.protocol, request_string, socket.inproc_url)
			if socket.protocol == "inproc" {
				err := socket.inproc_reconnect()
				if err != nil {
					return nil, fmt.Errorf("socket.inproc_reconnect: %w", err)
				}
			} else {
				err := socket.reconnect()
				if err != nil {
					return nil, fmt.Errorf("socket.reconnect: %w", err)
				}
			}
		}
	}
}

// Requests a message to the remote service.
// The socket parameter is the Request socket from this service.
// The request is the message.
func RequestReply[V SDS_Message](socket *Socket, request V) (key_value.KeyValue, error) {
	socket_type, err := socket.socket.GetType()
	if err != nil {
		return nil, fmt.Errorf("zmq socket get type: %w", err)
	}

	if socket_type != zmq.REQ && socket_type != zmq.DEALER {
		return nil, errors.New("invalid socket type for request-reply. Only REQ or DEALER is supported")
	}

	command_name := request.CommandName()

	request_timeout := parameter.RequestTimeout()

	request_string, err := request.ToString()
	if err != nil {
		return nil, fmt.Errorf("request.ToString: %w", err)
	}

	// we attempt requests for an infinite amount of time.
	for {
		//  We send a request, then we work to get a reply
		if _, err := socket.socket.SendMessage(request_string); err != nil {
			return nil, fmt.Errorf("failed to send the command '%s' to '%s'. socket error: %w", command_name, socket.remote_service.Name, err)
		}

		//  Poll socket for a reply, with timeout
		sockets, err := socket.poller.Poll(request_timeout)
		if err != nil {
			return nil, fmt.Errorf("failed to to send the command '%s' to '%s'. poll error: %w", command_name, socket.remote_service.Name, err)
		}

		//  Here we process a server reply and exit our loop if the
		//  reply is valid. If we didn't a reply we close the client
		//  socket and resend the request. We try a number of times
		//  before finally abandoning:

		if len(sockets) > 0 {
			// Wait for reply.
			r, err := socket.socket.RecvMessage(0)
			if err != nil {
				return nil, fmt.Errorf("failed to receive the command '%s' message from '%s'. socket error: %w", command_name, socket.remote_service.Name, err)
			}

			reply, err := message.ParseReply(r)
			if err != nil {
				return nil, fmt.Errorf("failed to parse the command '%s' reply from '%s'. gosds error %w", command_name, socket.remote_service.Name, err)
			}

			if !reply.IsOK() {
				return nil, fmt.Errorf("the command '%s' replied with a failure by '%s'. the reply error message: %s", command_name, socket.remote_service.Name, reply.Message)
			}

			return reply.Parameters, nil
		} else {
			fmt.Println("command '", command_name, "' wasn't replied by '", socket.remote_service.Name, "' in ", request_timeout, ", retrying...")
			err := socket.reconnect()
			if err != nil {
				return nil, fmt.Errorf("socket.reconnect: %w", err)
			}
		}
	}
}

// Create a new Socket on TCP protocol otherwise exit from the program
// The socket is the wrapper over zmq.REQ
func NewTcpSocket(remote_service *service.Service, client *credentials.Credentials) (*Socket, error) {
	sock, err := zmq.NewSocket(zmq.REQ)
	if err != nil {
		return nil, fmt.Errorf("zmq.NewSocket: %w", err)
	}

	new_socket := Socket{
		remote_service:     remote_service,
		client_credentials: client,
		socket:             sock,
		protocol:           "tcp",
	}
	err = new_socket.reconnect()
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	return &new_socket, nil
}

func InprocRequestSocket(url string) *Socket {
	sock, err := zmq.NewSocket(zmq.REQ)
	if err != nil {
		panic(err)
	}
	new_socket := Socket{
		socket:             sock,
		protocol:           "inproc",
		inproc_url:         url,
		client_credentials: nil,
	}
	err = new_socket.inproc_reconnect()
	if err != nil {
		panic(err)
	}

	return &new_socket
}

// Create a new Socket on TCP protocol otherwise exit from the program
// The socket is the wrapper over zmq.SUB
func NewTcpSubscriber(e *service.Service, client *credentials.Credentials) (*Socket, error) {
	socket, sockErr := zmq.NewSocket(zmq.SUB)
	if sockErr != nil {
		return nil, fmt.Errorf("new sub socket: %w", sockErr)
	}

	if client != nil {
		err := client.SetClientAuthCurve(socket, e.Credentials.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("credentials.SetClientAuthCurve: %w", err)
		}
	}

	conErr := socket.Connect(e.Url())
	if conErr != nil {
		return nil, fmt.Errorf("connect to broadcast: %w", conErr)
	}

	return &Socket{
		remote_service:     e,
		socket:             socket,
		client_credentials: client,
		protocol:           "tcp",
	}, nil
}
