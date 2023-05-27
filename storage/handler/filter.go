package handler

import (
	"fmt"

	"github.com/blocklords/sds/service/log"
	"github.com/blocklords/sds/storage/configuration"
	"github.com/blocklords/sds/storage/smartcontract"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/common/topic"

	"github.com/blocklords/sds/service/communication/command"
	"github.com/blocklords/sds/service/communication/message"
)

type FilterSmartcontractsRequest = topic.TopicFilter
type FilterSmartcontractsReply struct {
	Smartcontracts []*smartcontract.Smartcontract `json:"smartcontracts"`
	TopicStrings   []topic.TopicString            `json:"topic_strings"`
}

type FilterSmartcontractKeysRequest = topic.TopicFilter
type FilterSmartcontractKeysReply struct {
	SmartcontractKeys []smartcontract_key.Key `json:"smartcontract_keys"`
	TopicStrings      []topic.TopicString     `json:"topic_strings"`
}

func filter_organization(configurations *key_value.List, paths []string) *key_value.List {
	if len(paths) == 0 {
		return configurations
	}

	filtered := key_value.NewList()
	if configurations == nil {
		return filtered
	}

	list := configurations.List()
	for key, value := range list {
		conf := value.(*configuration.Configuration)

		for _, path := range paths {
			if conf.Topic.Organization == path {
				filtered.Add(key, value)
				break
			}
		}
	}

	return filtered
}

func filter_project(configurations *key_value.List, paths []string) *key_value.List {
	if len(paths) == 0 {
		return configurations
	}

	filtered := key_value.NewList()
	if configurations == nil {
		return filtered
	}

	list := configurations.List()
	for key, value := range list {
		conf := value.(*configuration.Configuration)

		for _, path := range paths {
			if conf.Topic.Project == path {
				filtered.Add(key, value)
				break
			}
		}
	}

	return filtered
}

func filter_network_id(configurations *key_value.List, paths []string) *key_value.List {
	if len(paths) == 0 {
		return configurations
	}

	filtered := key_value.NewList()
	if configurations == nil {
		return filtered
	}

	list := configurations.List()
	for key, value := range list {
		conf := value.(*configuration.Configuration)

		for _, path := range paths {
			if conf.Topic.NetworkId == path {
				filtered.Add(key, value)
				break
			}
		}
	}

	return filtered
}

func filter_group(configurations *key_value.List, paths []string) *key_value.List {
	if len(paths) == 0 {
		return configurations
	}

	filtered := key_value.NewList()
	if configurations == nil {
		return filtered
	}

	list := configurations.List()
	for key, value := range list {
		conf := value.(*configuration.Configuration)

		for _, path := range paths {
			if conf.Topic.Group == path {
				filtered.Add(key, value)
				break
			}
		}
	}

	return filtered
}

func filter_smartcontract_name(configurations *key_value.List, paths []string) *key_value.List {
	if len(paths) == 0 {
		return configurations
	}

	filtered := key_value.NewList()
	if configurations == nil {
		return filtered
	}

	list := configurations.List()
	for key, value := range list {
		conf := value.(*configuration.Configuration)

		for _, path := range paths {
			if conf.Topic.Smartcontract == path {
				filtered.Add(key, value)
				break
			}
		}
	}

	return filtered
}

func filter_configuration(configuration_list *key_value.List, topic_filter *topic.TopicFilter) []*configuration.Configuration {
	list := key_value.NewList()

	if len(topic_filter.Organizations) != 0 {
		list = filter_organization(configuration_list, topic_filter.Organizations)
	}

	if len(topic_filter.Projects) != 0 {
		list = filter_project(list, topic_filter.Projects)
	}

	if len(topic_filter.NetworkIds) != 0 {
		list = filter_network_id(list, topic_filter.NetworkIds)
	}

	if len(topic_filter.Groups) != 0 {
		list = filter_group(list, topic_filter.Groups)
	}

	if len(topic_filter.Smartcontracts) != 0 {
		list = filter_smartcontract_name(list, topic_filter.Smartcontracts)
	}

	configs := make([]*configuration.Configuration, list.Len())

	i := 0
	for _, value := range list.List() {
		conf := value.(*configuration.Configuration)
		configs[i] = conf
		i++
	}

	return configs
}

func filter_smartcontract(
	configurations []*configuration.Configuration,
	list *key_value.List) ([]*smartcontract.Smartcontract, []topic.TopicString, error) {

	smartcontracts := make([]*smartcontract.Smartcontract, 0)
	topic_strings := make([]topic.TopicString, 0)

	for _, conf := range configurations {
		key, err := smartcontract_key.New(conf.Topic.NetworkId, conf.Address)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create smartcontract key: %w", err)
		}

		value, err := list.Get(key)
		if err != nil {
			fmt.Println("not found")
			continue
		}
		sm := value.(*smartcontract.Smartcontract)

		smartcontracts = append(smartcontracts, sm)
		topic_strings = append(topic_strings, conf.Topic.ToString(topic.SMARTCONTRACT_LEVEL))
	}

	return smartcontracts, topic_strings, nil
}

/*
Return list of smartcontracts by given filter topic.

Algorithm

 1. the Package configuration has a function that returns amount of
    smartcontracts that matches the filter.
 2. If the amount is 0, then return empty result.
 3. the smartcontract package has a function that returns
    list of smartcontracts by filter.
    The smartcontract package accepts the db_query from configuration config.
 4. return list of smartcontracts back
*/
func SmartcontractFilter(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	var topic_filter FilterSmartcontractKeysRequest
	err := request.Parameters.ToInterface(&topic_filter)
	if err != nil {
		return message.Fail("failed to parse data")
	}

	all_configurations := parameters[3].(*key_value.List)
	configurations := filter_configuration(all_configurations, &topic_filter)
	if len(configurations) == 0 {
		reply := FilterSmartcontractsReply{
			Smartcontracts: []*smartcontract.Smartcontract{},
			TopicStrings:   []topic.TopicString{},
		}
		reply_message, err := command.Reply(&reply)
		if err != nil {
			return message.Fail("failed to reply: " + err.Error())
		}
		return reply_message
	}

	all_smartcontracts := parameters[2].(*key_value.List)
	smartcontracts, topic_strings, err := filter_smartcontract(configurations, all_smartcontracts)
	if err != nil {
		return message.Fail("failed to reply: " + err.Error())
	}

	reply := FilterSmartcontractsReply{
		Smartcontracts: smartcontracts,
		TopicStrings:   topic_strings,
	}
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("failed to reply: " + err.Error())
	}
	return reply_message
}

// returns smartcontract keys and topic of the smartcontract
// by given topic filter
//
//	returns {
//			"smartcontract_keys" (where key is smartcontract key, value is a topic string)
//	}
func SmartcontractKeyFilter(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	var topic_filter FilterSmartcontractKeysRequest
	err := request.Parameters.ToInterface(&topic_filter)
	if err != nil {
		return message.Fail("failed to parse data")
	}

	all_configurations := parameters[3].(*key_value.List)
	configurations := filter_configuration(all_configurations, &topic_filter)
	if len(configurations) == 0 {
		reply := FilterSmartcontractKeysReply{
			SmartcontractKeys: []smartcontract_key.Key{},
			TopicStrings:      []topic.TopicString{},
		}
		reply_message, err := command.Reply(&reply)
		if err != nil {
			return message.Fail("failed to reply: " + err.Error())
		}
		return reply_message
	}

	all_smartcontracts := parameters[2].(*key_value.List)
	smartcontracts, topic_strings, err := filter_smartcontract(configurations, all_smartcontracts)
	if err != nil {
		return message.Fail("failed to reply: " + err.Error())
	}

	keys := make([]smartcontract_key.Key, len(smartcontracts))
	for i, smartcontract := range smartcontracts {
		keys[i] = smartcontract.SmartcontractKey
	}

	reply := FilterSmartcontractKeysReply{
		SmartcontractKeys: keys,
		TopicStrings:      topic_strings,
	}
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("failed to reply: " + err.Error())
	}
	return reply_message
}
