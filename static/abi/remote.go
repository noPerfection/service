// The remote.go contains the functions that interact with the Abi in a remote service
package abi

import (
	"errors"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
)

// Sends the ABI information to the remote SDS Static.
func Set(socket *remote.Socket, body interface{}) (key_value.KeyValue, error) {
	// Send hello.
	request := message.Request{
		Command: "abi_set",
		Parameters: map[string]interface{}{
			"body": body,
		},
	}

	return socket.RequestRouter(service.STATIC, &request)
}

// Returns the abi from the remote server
func Get(socket *remote.Socket, key smartcontract_key.Key) (*Abi, error) {
	// Send hello.
	request := message.Request{
		Command: "abi_get",
		Parameters: map[string]interface{}{
			"network_id": key.NetworkId,
			"address":    key.Address,
		},
	}

	raw_params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, err
	}

	abi_bytes, ok := raw_params["body"]
	if !ok {
		return nil, errors.New("missing 'abi' parameter from the SDS Static 'abi_get' command")
	}

	new_abi, err := NewFromBytes([]byte(abi_bytes.(string)))
	if err != nil {
		return nil, err
	}

	return new_abi, nil
}
