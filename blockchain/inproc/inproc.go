package inproc

import (
	"fmt"

	zmq "github.com/pebbe/zmq4"
)

// returns the url for a blockchain clients manager
func BlockchainManagerUrl(network_id string) string {
	return "inproc://spaghetti_" + network_id
}

// returns the categorizer manager url
func CategorizerManagerUrl(network_id string) string {
	return "inproc://cat_" + network_id
}

func CategorizerManagerSocket(network_id string) (*zmq.Socket, error) {
	sock, err := zmq.NewSocket(zmq.PUSH)
	if err != nil {
		return nil, fmt.Errorf("zmq error for new push socket: %w", err)
	}

	url := CategorizerManagerUrl(network_id)
	if err := sock.Bind(url); err != nil {
		return nil, fmt.Errorf("trying to create categorizer for network id %s: %v", network_id, err)
	}

	return sock, nil
}
