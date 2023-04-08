// Package abi creates the ethereum ABI from [static/abi.Abi]
package abi

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	static_abi "github.com/blocklords/sds/static/abi"
	eth_common "github.com/ethereum/go-ethereum/common"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

// Abi struct is used for EVM based categorizer.
//
// It has the utility function specific for EVM to decode.
// raw data from blockchain into categorized data.
//
// Its the wrapper over the SDS Static abi.
type Abi struct {
	abi abi.ABI // interface
}

// Creates a categorizer abi.
// It adds an ethereum abi layer on top of the static abi.
func NewFromStatic(static_abi *static_abi.Abi) (*Abi, error) {
	abi_obj := Abi{}

	if err := json.Unmarshal(static_abi.Bytes, &abi_obj.abi); err != nil {
		return nil, fmt.Errorf("failed to decompose static abi to geth abi: %w", err)
	}

	return &abi_obj, nil
}

// Returns the list of all indexed
// outputs of the event
func get_indexed(inputs abi.Arguments) abi.Arguments {
	ret := make(abi.Arguments, 0)
	for _, arg := range inputs {
		if arg.Indexed {
			ret = append(ret, arg)
		}
	}
	return ret
}

// List of events by their event id
// There could be multiple events
// with the same id.
//
// however the values could be changed
// at the indexing.
func (a *Abi) get_events(event_id string) []abi.Event {
	events := make([]abi.Event, 0)
	for _, event := range a.abi.Events {
		if strings.EqualFold(event_id, event.ID.String()) {
			events = append(events, event)
		}
	}

	return events
}

// Parse the data and indexed data (topics) to key-value
//
// To understand the event data and event topics refer to the Ethereum documentation.
func (a *Abi) DecodeLog(topics []string, data string) (string, map[string]interface{}, error) {
	if len(topics) == 0 {
		return "", nil, fmt.Errorf("anonymous events are not supported")
	}

	topic_hashes := make([]eth_common.Hash, len(topics)-1)
	var event_id eth_common.Hash
	for i, topic := range topics {
		if i == 0 {
			event_id = eth_common.HexToHash(topic)
		} else {
			topic_hashes[i-1] = eth_common.HexToHash(topic)
		}
	}

	topic_outputs := make(map[string]interface{}, 0)

	data_outputs := make(map[string]interface{}, 0)
	events := a.get_events(event_id.String())
	if len(events) == 0 {
		return "", nil, fmt.Errorf("no event in abi: %v", event_id)
	}
	for _, event := range events {
		indexed := get_indexed(event.Inputs)
		if len(indexed) == len(topic_hashes) {
			if len(data) > 0 {
				bytes, err := hex.DecodeString(data)
				if err != nil {
					return "", nil, fmt.Errorf("error decoding data string to bytes: %w", err)
				}
				err = event.Inputs.NonIndexed().UnpackIntoMap(data_outputs, bytes)
				if err != nil {
					return "", nil, fmt.Errorf("parsing event %s for data %s error: %w", event.RawName, data, err)
				}
			}

			err := abi.ParseTopicsIntoMap(topic_outputs, indexed, topic_hashes)
			if err != nil {
				return "", nil, fmt.Errorf("event %s for %v topics parsing error: %w", event.RawName, topics, err)
			}

			// merge topics and data
			for key, value := range topic_outputs {
				data_outputs[key] = value
			}

			return event.RawName, data_outputs, nil
		}
	}

	return "", nil, fmt.Errorf("failed to decode the event. No topic amount")
}
