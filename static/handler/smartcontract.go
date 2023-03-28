package handler

import (
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/db"
	"github.com/blocklords/sds/static/configuration"
	"github.com/blocklords/sds/static/smartcontract"

	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/common/topic"

	"github.com/blocklords/sds/app/command"
	"github.com/blocklords/sds/app/remote/message"
)

type FilterSmartcontractsRequest = topic.TopicFilter
type FilterSmartcontractsReply struct {
	Smartcontracts []smartcontract.Smartcontract `json:"smartcontracts"`
	Topics         []string                      `json:"topics"`
}

type FilterSmartcontractKeysRequest = topic.TopicFilter
type FilterSmartcontractKeysReply struct {
	SmartcontractKeys map[string]string `json:"smartcontract_keys"`
}

type SetSmartcontractRequest = smartcontract.Smartcontract
type SetSmartcontractReply = smartcontract.Smartcontract
type GetSmartcontractRequest = smartcontract_key.Key
type GetSmartcontractReply = smartcontract.Smartcontract

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
	db_con := parameters[0].(*db.Database)

	var topic_filter FilterSmartcontractKeysRequest
	err := request.Parameters.ToInterface(&topic_filter)
	if err != nil {
		return message.Fail("failed to parse data")
	}

	query, query_parameters := configuration.QueryFilterSmartcontract(&topic_filter)

	smartcontracts, topics, err := smartcontract.FilterFromDatabase(db_con, query, query_parameters)
	if err != nil {
		return message.Fail("failed to filter smartcontracts by the topic filter:" + err.Error())
	} else if len(smartcontracts) == 0 {
		return message.Fail("no matching smartcontracts for the topic filter " + topic_filter.ToString())
	}

	// list of smartcontracts (map)
	topic_strings := make([]string, len(smartcontracts))

	for i := range smartcontracts {
		topic_strings[i] = topics[i].ToString(topic.SMARTCONTRACT_LEVEL)
	}

	reply := FilterSmartcontractsReply{
		Smartcontracts: smartcontracts,
		Topics:         topic_strings,
	}
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("failed to reply")
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
	db_con := parameters[0].(*db.Database)

	var topic_filter FilterSmartcontractsRequest
	err := request.Parameters.ToInterface(&topic_filter)
	if err != nil {
		return message.Fail("failed to parse data")
	}
	query, query_parameters := configuration.QueryFilterSmartcontract(&topic_filter)

	smartcontract_keys, topics, err := smartcontract.FilterKeysFromDatabase(db_con, query, query_parameters)
	if err != nil {
		return message.Fail(err.Error())
	}

	blob := make(map[string]string, len(smartcontract_keys))
	for i, key := range smartcontract_keys {
		blob[key.ToString()] = topics[i].ToString(topic.SMARTCONTRACT_LEVEL)
	}

	reply := FilterSmartcontractKeysReply{
		SmartcontractKeys: blob,
	}
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("failed to reply")
	}

	return reply_message
}

// Register a new smartcontract. It means we are adding smartcontract parameters into
// static_smartcontract.
// Requires abi_id parameter. First call abi_register method first.
func SmartcontractRegister(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	db_con := parameters[0].(*db.Database)

	var sm SetSmartcontractRequest
	err := request.Parameters.ToInterface(&sm)
	if err != nil {
		return message.Fail("failed to parse data")
	}

	var reply SetSmartcontractReply = sm
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("failed to reply")
	}

	if smartcontract.ExistInDatabase(db_con, sm.SmartcontractKey) {
		return reply_message
	}

	if err = smartcontract.SetInDatabase(db_con, &sm); err != nil {
		return message.Fail("Smartcontract saving in the database failed: " + err.Error())
	}

	return reply_message
}

// Returns configuration and smartcontract information related to the configuration
func SmartcontractGet(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	db_con := parameters[0].(*db.Database)

	var key GetSmartcontractRequest
	err := request.Parameters.ToInterface(&key)
	if err != nil {
		return message.Fail("failed to parse data")
	}

	sm, err := smartcontract.GetFromDatabase(db_con, key)
	if err != nil {
		return message.Fail("Failed to get smartcontract from database: " + err.Error())
	}

	var reply SetSmartcontractReply = *sm
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("failed to reply")
	}

	return reply_message
}
