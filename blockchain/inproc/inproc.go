package inproc

import (
	"fmt"

	zmq "github.com/pebbe/zmq4"
)

// Returns the endpoint to the blockchain clients manager.
// Use this endpoint in the req socket to interact with the blockchain nodes.
// The interaction goes through blockchain/<network id>/client.
func ClientEndpoint(network_id string) string {
	return "inproc://spaghetti_" + network_id
}

// Returns the recent smartcontract categorizer
// manager url
func RecentCategorizerEndpoint(network_id string) string {
	return "inproc://cat_recent_" + network_id
}

// Returns the old smartcontract categorizer
// manager url
func OldCategorizerEndpoint(network_id string) string {
	return "inproc://cat_old_" + network_id
}

// Returns the categorizer manager url
func CategorizerEndpoint(network_id string) string {
	return "inproc://cat_" + network_id
}

func NewInprocPusher(url string) (*zmq.Socket, error) {
	sock, err := zmq.NewSocket(zmq.PUSH)
	if err != nil {
		return nil, fmt.Errorf("zmq error for new push socket: %w", err)
	}

	if err := sock.Connect(url); err != nil {
		return nil, fmt.Errorf("socket.Connect: %s: %w", url, err)
	}

	return sock, nil
}

func RecentCategorizerManagerSocket(network_id string) (*zmq.Socket, error) {
	url := RecentCategorizerEndpoint(network_id)
	return NewInprocPusher(url)
}

func OldCategorizerManagerSocket(network_id string) (*zmq.Socket, error) {
	url := OldCategorizerEndpoint(network_id)
	return NewInprocPusher(url)
}

func CategorizerManagerSocket(network_id string) (*zmq.Socket, error) {
	url := CategorizerEndpoint(network_id)
	return NewInprocPusher(url)
}
