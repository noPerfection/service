// Package inproc defines the client sockets and socket endpoints
// for sub blockchain processes.
package inproc

import (
	"fmt"

	zmq "github.com/pebbe/zmq4"
)

// ClientEndpoint returns the endpoint to the blockchain clients manager.
// Use this endpoint in the req socket to interact with the blockchain nodes.
// The interaction goes through blockchain/<network id>/client.
func ClientEndpoint(network_id string) string {
	return "inproc://blockchain_" + network_id
}

// RecentCategorizerEndpoint returns the recent smartcontract categorizer
// manager url
//
// Used in evm sub process.
func RecentCategorizerEndpoint(network_id string) string {
	return "inproc://cat_recent_" + network_id
}

// RecentCategorizerReplyEndpoint returns reply controller running in the recent categorizer
//
// Used in evm sub process.
func RecentCategorizerReplyEndpoint(network_id string) string {
	return "inproc://cat_recent_rep_" + network_id
}

// Returns the old smartcontract categorizer
// manager url
//
// Used in evm sub process.
func OldCategorizerEndpoint(network_id string) string {
	return "inproc://cat_old_" + network_id
}

// CategorizerEndpoint returns the sub categorizer manager url
func CategorizerEndpoint(network_id string) string {
	return "inproc://cat_" + network_id
}

// NewInprocPusher creates a pusher for the url
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

// RecentCategorizerManagerSocket returns the client socket
// that is accessed to the [blockchain/evm/categorizer/recent] pull controller
func RecentCategorizerManagerSocket(network_id string) (*zmq.Socket, error) {
	url := RecentCategorizerEndpoint(network_id)
	return NewInprocPusher(url)
}

// OldCategorizerManagerSocket returns the client socket
// that is accessed to the [blockchain/evm/categorizer/old] pull controller
func OldCategorizerManagerSocket(network_id string) (*zmq.Socket, error) {
	url := OldCategorizerEndpoint(network_id)
	return NewInprocPusher(url)
}

// CategorizerManagerSocket returns the client socket
// that is accessed to the [blockchain/<network type>/categorizer] pull controller
func CategorizerManagerSocket(network_id string) (*zmq.Socket, error) {
	url := CategorizerEndpoint(network_id)
	return NewInprocPusher(url)
}
