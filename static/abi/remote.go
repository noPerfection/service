// The remote.go contains the functions that interact with the Abi in a remote service
package abi

import (
	"errors"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
)

// Sends the ABI information to the remote SDS Static.
func Set(socket *remote.Socket, body interface{}) (map[string]interface{}, error) {
	// Send hello.
	request := message.Request{
		Command: "abi_register",
		Parameters: map[string]interface{}{
			"abi": body,
		},
	}

	return socket.RequestRemoteService(&request)
}

// Returns the abi from the remote server
func Get(socket *remote.Socket, network_id string, address string) (*Abi, error) {
	// Send hello.
	request := message.Request{
		Command: "abi_get",
		Parameters: map[string]interface{}{
			"network_id": network_id,
			"address":    address,
		},
	}

	raw_params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, err
	}

	abi_bytes, ok := raw_params["abi"]
	if !ok {
		return nil, errors.New("missing 'abi' parameter from the SDS Static 'abi_get' command")
	}

	new_abi, err := NewFromBytes([]byte(abi_bytes.(string)))
	if err != nil {
		return nil, err
	}

	return new_abi, nil
}
