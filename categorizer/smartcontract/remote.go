package smartcontract

import (
	"fmt"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/common/data_type/key_value"
)

// Sends a command to the remote SDS Categorizer about regitration of this smartcontract.
func RemoteSet(b *Smartcontract, socket *remote.Socket) error {
	// Send hello.
	request := message.Request{
		Command:    "smartcontract_set",
		Parameters: key_value.Empty().Set("smartcontract", b),
	}

	_, err := socket.RequestRouter(service.CATEGORIZER, &request)
	if err != nil {
		return err
	}

	return nil
}

// Returns a smartcontract information from the remote SDS Categorizer.
func RemoteSmartcontract(socket *remote.Socket, network_id string, address string) (*Smartcontract, error) {
	// Send hello.
	request := message.Request{
		Command:    "smartcontract_get",
		Parameters: key_value.Empty().Set("network_id", network_id).Set("address", address),
	}
	raw_params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, fmt.Errorf("smartcontract_get remote request: %w", err)
	}
	params := key_value.New(raw_params)

	smartcontract, err := params.GetKeyValue("smartcontract")
	if err != nil {
		return nil, fmt.Errorf("parameter.GetKeyValue(`smartcontract`): %w", err)
	}

	sm, err := New(smartcontract)
	if err != nil {
		return nil, fmt.Errorf("New: %w", err)
	}

	return sm, nil
}

// Returns all smartcontracts from SDS Categorizer
func RemoteSmartcontracts(socket *remote.Socket) ([]*Smartcontract, error) {
	// Send hello.
	request := message.Request{
		Command:    "smartcontract_get_all",
		Parameters: key_value.Empty(),
	}

	raw_params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, fmt.Errorf("smartcontract_get_all remote request: %w", err)
	}
	params := key_value.New(raw_params)

	raw_smartcontracts, err := params.GetKeyValueList("smartcontracts")
	if err != nil {
		return nil, fmt.Errorf("GetKeyValueList(`smartcontracts`): %w", err)
	}

	smartcontracts := make([]*Smartcontract, len(raw_smartcontracts))
	for i, raw := range raw_smartcontracts {
		smartcontract, err := New(raw)
		if err != nil {
			return nil, fmt.Errorf("raw_smartcontracts[%d] New: %w", i, err)
		}

		smartcontracts[i] = smartcontract
	}

	return smartcontracts, nil
}
