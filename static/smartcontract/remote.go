package smartcontract

import (
	"errors"
	"fmt"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/topic"
	"github.com/blocklords/sds/static/smartcontract/key"
)

// Returns list of smartcontracts by topic filter in remote Static service
// also the topic path of the smartcontract
func RemoteSmartcontracts(socket *remote.Socket, tf *topic.TopicFilter) ([]*Smartcontract, []string, error) {
	kv, err := key_value.NewFromInterface(tf)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to serialize topic filter: %v", err)
	}

	request := message.Request{
		Command:    "smartcontract_filter",
		Parameters: key_value.Empty().Set("topic_filter", kv),
	}

	raw_params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, nil, err
	}
	params := key_value.New(raw_params)

	raw_smartcontracts, err := params.GetKeyValueList("smartcontracts")
	if err != nil {
		return nil, nil, err
	}
	topic_strings, err := params.GetStringList("topics")
	if err != nil {
		return nil, nil, err
	}
	if len(raw_smartcontracts) != len(topic_strings) {
		return nil, nil, errors.New("the returned amount of topic strings mismatch with smartcontracts")
	}
	var smartcontracts []*Smartcontract = make([]*Smartcontract, len(raw_smartcontracts))
	for i, raw_smartcontract := range raw_smartcontracts {
		smartcontract, err := New(raw_smartcontract)
		if err != nil {
			return nil, nil, err
		}
		smartcontracts[i] = smartcontract
	}

	return smartcontracts, topic_strings, nil
}

// returns list of smartcontract keys by topic filter
func RemoteSmartcontractKeys(socket *remote.Socket, tf *topic.TopicFilter) (key.KeyToTopicString, error) {
	kv, err := key_value.NewFromInterface(tf)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize topic filter: %v", err)
	}

	// Send hello.
	request := message.Request{
		Command:    "smartcontract_key_filter",
		Parameters: key_value.Empty().Set("topic_filter", kv),
	}
	raw_params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, err
	}
	params := key_value.New(raw_params)

	raw_keys, err := params.GetKeyValue("smartcontract_keys")
	if err != nil {
		return nil, err
	}
	var keys key.KeyToTopicString = make(key.KeyToTopicString, len(raw_keys))
	for raw_key, raw_value := range raw_keys {
		topic_string, ok := raw_value.(string)
		if !ok {
			return nil, errors.New("one of the topic strings is not in the string format")
		}
		keys[key.NewFromString(raw_key)] = topic_string
	}

	return keys, nil
}

// returns smartcontract by smartcontract key from SDS Static
func RemoteSmartcontract(socket *remote.Socket, network_id string, address string) (*Smartcontract, error) {
	// Send hello.
	request := message.Request{
		Command:    "smartcontract_get",
		Parameters: key_value.Empty().Set("network_id", network_id).Set("address", address),
	}
	raw_params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, err
	}
	params := key_value.New(raw_params)

	raw_smartcontract, err := params.GetKeyValue("smartcontract")
	if err != nil {
		return nil, err
	}
	return New(raw_smartcontract)
}

func RemoteSmartcontractRegister(socket *remote.Socket, s *Smartcontract) (string, error) {
	json, err := key_value.NewFromInterface(s)
	if err != nil {
		return "", fmt.Errorf("preparing request message failed. failed to serialize static.Smartcontract to json while: %v", err)
	}

	// Send hello.
	request := message.Request{
		Command:    "smartcontract_register",
		Parameters: json,
	}

	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return "", err
	}

	return params.GetString("address")
}
