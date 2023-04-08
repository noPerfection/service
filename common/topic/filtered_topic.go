// Package topic defines the special kind of data type called topic and topic filter.
// The topics are replacing the smartcontract addresses in order to detect the smartcontract
// that user wants to interact with.
//
// For example, if the user wants to interact with Crowns crypto currency
// on Ethereum network, then user will need to know the Crowns ABI interface,
// as well as the smartcontract address.
//
// In SDS its replaced with the Topic.
// Define the topic something like:
//
//		topic := topic.Topic{
//			Organization: "seascape",
//	     	NetworkId: "1",
//	     	Smartcontract: "Crowns"
//		}
//
// For example, use the topic in SDK to read the data from categorizer.
// Viola, we don't need to remember smartcontract address.
package topic

import (
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
)

// Filter unlike Topic can omit the parameters
// Allows to define list of smartcontracts that match the topic filter.
//
// Which means users can interact with multiple smartcontracts at once.
type TopicFilter struct {
	Organizations  []string `json:"o,omitempty"`
	Projects       []string `json:"p,omitempty"`
	NetworkIds     []string `json:"n,omitempty"`
	Groups         []string `json:"g,omitempty"`
	Smartcontracts []string `json:"s,omitempty"`
	Events         []string `json:"e,omitempty"`
}

// NewFilterTopic from the given parameters
func NewFilterTopic(o []string, p []string, n []string, g []string, s []string, e []string) TopicFilter {
	return TopicFilter{
		Organizations:  o,
		Projects:       p,
		NetworkIds:     n,
		Groups:         g,
		Smartcontracts: s,
		Events:         e,
	}
}

// convert properties to string
func reduce_properties(properties []string) string {
	str := ""
	for i, v := range properties {
		if i != 0 {
			str += ","
		}
		str += v
	}

	return str
}

func (f *TopicFilter) has_nested_level(level uint8) bool {
	switch level {
	case ORGANIZATION_LEVEL:
		if !f.has_nested_level(PROJECT_LEVEL) {
			return len(f.Organizations) != 0
		}
		return true
	case PROJECT_LEVEL:
		if !f.has_nested_level(NETWORK_ID_LEVEL) {
			return len(f.Projects) != 0
		}
		return true
	case NETWORK_ID_LEVEL:
		if !f.has_nested_level(GROUP_LEVEL) {
			return len(f.NetworkIds) != 0
		}
		return true
	case GROUP_LEVEL:
		if !f.has_nested_level(SMARTCONTRACT_LEVEL) {
			return len(f.Groups) != 0
		}
		return true
	case SMARTCONTRACT_LEVEL:
		if !f.has_nested_level(FULL_LEVEL) {
			return len(f.Smartcontracts) != 0
		}
		return true
	case FULL_LEVEL:
		return len(f.Events) != 0
	}
	return false
}

// Convert the topic filter object to the topic filter string.
func (t *TopicFilter) ToString() TopicString {
	str := ""
	if len(t.Organizations) > 0 {
		str += "o:" + reduce_properties(t.Organizations)
		if t.has_nested_level(ORGANIZATION_LEVEL) {
			str += ";"
		}
	}
	if len(t.Projects) > 0 {
		str += "p:" + reduce_properties(t.Projects)
		if t.has_nested_level(PROJECT_LEVEL) {
			str += ";"
		}
	}
	if len(t.NetworkIds) > 0 {
		str += "n:" + reduce_properties(t.NetworkIds)
		if t.has_nested_level(NETWORK_ID_LEVEL) {
			str += ";"
		}
	}
	if len(t.Groups) > 0 {
		str += "g:" + reduce_properties(t.Groups)
		if t.has_nested_level(GROUP_LEVEL) {
			str += ";"
		}
	}
	if len(t.Smartcontracts) > 0 {
		str += "s:" + reduce_properties(t.Smartcontracts)
		if t.has_nested_level(SMARTCONTRACT_LEVEL) {
			str += ";"
		}
	}
	if len(t.Events) > 0 {
		str += "e:" + reduce_properties(t.Events)
	}

	return TopicString(str)
}

// If the given key value has a "topic_filter" key
// then the value should be topic filter
func NewFromKeyValueParameter(parameters key_value.KeyValue) (*TopicFilter, error) {
	topic_filter_map, err := parameters.GetKeyValue("topic_filter")
	if err != nil {
		return nil, fmt.Errorf("missing `topic_filter` parameter")
	}

	var topic_filter TopicFilter
	err = topic_filter_map.ToInterface(&topic_filter)

	if err != nil {
		return nil, fmt.Errorf("failed to convert the value to TopicFilter: %w", err)
	}

	return &topic_filter, nil
}
