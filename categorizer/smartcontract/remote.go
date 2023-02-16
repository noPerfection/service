package smartcontract

import (
	"github.com/blocklords/gosds/message"
	"github.com/blocklords/gosds/remote"
)

// Sends a command to the remote SDS Categorizer about regitration of this smartcontract.
func RemoteSet(b *Smartcontract, socket *remote.Socket) error {
	// Send hello.
	request := message.Request{
		Command:    "smartcontract_set",
		Parameters: b.ToJSON(),
	}

	_, err := socket.RequestRemoteService(&request)
	if err != nil {
		return err
	}

	return nil
}

// Returns a smartcontract information from the remote SDS Categorizer.
func RemoteSmartcontract(socket *remote.Socket, network_id string, address string) (*Smartcontract, error) {
	// Send hello.
	request := message.Request{
		Command: "smartcontract_get",
		Parameters: map[string]interface{}{
			"network_id": network_id,
			"address":    address,
		},
	}
	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, err
	}

	smartcontract, err := message.GetMap(params, "smartcontract")
	if err != nil {
		return nil, err
	}

	return New(smartcontract)
}

// Returns all smartcontracts from SDS Categorizer
func RemoteSmartcontracts(socket *remote.Socket) ([]*Smartcontract, error) {
	// Send hello.
	request := message.Request{
		Command:    "smartcontract_get_all",
		Parameters: map[string]interface{}{},
	}

	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, err
	}

	raw_smartcontracts, err := message.GetMapList(params, "smartcontracts")
	if err != nil {
		return nil, err
	}

	smartcontracts := make([]*Smartcontract, len(raw_smartcontracts))
	for i, raw := range raw_smartcontracts {
		smartcontract, err := New(raw)
		if err != nil {
			return nil, err
		}

		smartcontracts[i] = smartcontract
	}

	return smartcontracts, nil
}
