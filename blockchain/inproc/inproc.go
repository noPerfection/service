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

// RecentIndexerEndpoint returns the recent smartcontract indexer
// manager url
//
// Used in evm sub process.
func RecentIndexerEndpoint(network_id string) string {
	return "inproc://cat_recent_" + network_id
}

// RecentIndexerReplyEndpoint returns reply controller running in the recent indexer
//
// Used in evm sub process.
func RecentIndexerReplyEndpoint(network_id string) string {
	return "inproc://cat_recent_rep_" + network_id
}

// Returns the old smartcontract indexer
// manager url
//
// Used in evm sub process.
func OldIndexerEndpoint(network_id string) string {
	return "inproc://cat_old_" + network_id
}

// IndexerEndpoint returns the sub indexer manager url
func IndexerEndpoint(network_id string) string {
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

// RecentIndexerManagerSocket returns the client socket
// that is accessed to the [blockchain/evm/indexer/recent] pull controller
func RecentIndexerManagerSocket(network_id string) (*zmq.Socket, error) {
	url := RecentIndexerEndpoint(network_id)
	return NewInprocPusher(url)
}

// OldIndexerManagerSocket returns the client socket
// that is accessed to the [blockchain/evm/indexer/old] pull controller
func OldIndexerManagerSocket(network_id string) (*zmq.Socket, error) {
	url := OldIndexerEndpoint(network_id)
	return NewInprocPusher(url)
}

// IndexerManagerSocket returns the client socket
// that is accessed to the [blockchain/<network type>/indexer] pull controller
func IndexerManagerSocket(network_id string) (*zmq.Socket, error) {
	url := IndexerEndpoint(network_id)
	return NewInprocPusher(url)
}
