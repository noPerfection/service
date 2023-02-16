package smartcontract

import (
	"errors"

	"github.com/blocklords/gosds/message"
	"github.com/blocklords/gosds/remote"
	"github.com/blocklords/gosds/static/smartcontract/key"
	"github.com/blocklords/gosds/topic"
)

// Returns list of smartcontracts by topic filter in remote Static service
// also the topic path of the smartcontract
func RemoteSmartcontracts(socket *remote.Socket, tf *topic.TopicFilter) ([]*Smartcontract, []string, error) {
	request := message.Request{
		Command: "smartcontract_filter",
		Parameters: map[string]interface{}{
			"topic_filter": tf.ToJSON(),
		},
	}
	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, nil, err
	}

	raw_smartcontracts, err := message.GetMapList(params, "smartcontracts")
	if err != nil {
		return nil, nil, err
	}
	topic_strings, err := message.GetStringList(params, "topics")
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
	// Send hello.
	request := message.Request{
		Command: "smartcontract_key_filter",
		Parameters: map[string]interface{}{
			"topic_filter": tf.ToJSON(),
		},
	}
	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, err
	}

	raw_keys, err := message.GetMap(params, "smartcontract_keys")
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

	raw_smartcontract, err := message.GetMap(params, "smartcontract")
	if err != nil {
		return nil, err
	}
	return New(raw_smartcontract)
}

func RemoteSmartcontractRegister(socket *remote.Socket, s *Smartcontract) (string, error) {
	// Send hello.
	request := message.Request{
		Command:    "smartcontract_register",
		Parameters: s.ToJSON(),
	}

	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return "", err
	}

	return message.GetString(params, "address")
}
