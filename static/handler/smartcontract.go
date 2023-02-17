package handler

import (
	"github.com/blocklords/gosds/db"
	"github.com/blocklords/gosds/static/configuration"
	"github.com/blocklords/gosds/static/smartcontract"

	"github.com/blocklords/gosds/common/data_type"
	"github.com/blocklords/gosds/common/data_type/key_value"
	"github.com/blocklords/gosds/common/topic"

	"github.com/blocklords/gosds/app/remote/message"
)

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
func SmartcontractFilter(dbCon *db.Database, request message.Request) message.Reply {
	topic_filter_map, err := request.Parameters.GetKeyValue("topic_filter")
	if err != nil {
		return message.Fail(err.Error())
	}
	topic_filter, err := topic.ParseJSONToTopicFilter(topic_filter_map)
	if err != nil {
		return message.Fail(err.Error())
	}

	query, parameters := configuration.QueryFilterSmartcontract(topic_filter)

	smartcontracts, topics, err := smartcontract.GetFromDatabaseFilterBy(dbCon, query, parameters)
	if err != nil {
		return message.Fail(err.Error())
	}

	// list of smartcontracts (map)
	topic_strings := make([]string, len(smartcontracts))

	for i := range smartcontracts {
		topic_strings[i] = topics[i].ToString(topic.SMARTCONTRACT_LEVEL)
	}

	reply := message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"smartcontracts": data_type.ToMapList(smartcontracts),
			"topics":         topic_strings,
		}),
	}
	return reply
}

// returns smartcontract keys and topic of the smartcontract
// by given topic filter
//
//	returns {
//			"smartcontract_keys" (where key is smartcontract key, value is a topic string)
//	}
func SmartcontractKeyFilter(dbCon *db.Database, request message.Request) message.Reply {
	topic_filter_map, err := request.Parameters.GetKeyValue("topic_filter")
	if err != nil {
		return message.Fail(err.Error())
	}
	topic_filter, err := topic.ParseJSONToTopicFilter(topic_filter_map)
	if err != nil {
		return message.Fail(err.Error())
	}

	query, parameters := configuration.QueryFilterSmartcontract(topic_filter)

	smartcontracts, topics, err := smartcontract.GetFromDatabaseFilterBy(dbCon, query, parameters)
	if err != nil {
		return message.Fail(err.Error())
	}

	blob := make(map[string]string, len(smartcontracts))
	for i, s := range smartcontracts {
		blob[string(s.Key())] = topics[i].ToString(topic.SMARTCONTRACT_LEVEL)
	}

	reply := message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"smartcontract_keys": blob,
		}),
	}
	return reply
}

// Register a new smartcontract. It means we are adding smartcontract parameters into
// static_smartcontract.
// Requires abi_hash parameter. First call abi_register method first.
func SmartcontractRegister(dbCon *db.Database, request message.Request) message.Reply {
	sm, err := smartcontract.New(request.Parameters)
	if err != nil {
		return message.Fail(err.Error())
	}

	reply := message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"network_id": sm.NetworkId,
			"address":    sm.Address,
		}),
	}

	if smartcontract.ExistInDatabase(dbCon, sm.NetworkId, sm.Address) {
		return reply
	}

	if err = smartcontract.SetInDatabase(dbCon, sm); err != nil {
		return message.Fail("Smartcontract saving in the database failed: " + err.Error())
	}

	return reply
}

// Returns configuration and smartcontract information related to the configuration
func SmartcontractGet(db *db.Database, request message.Request) message.Reply {
	network_id, err := request.Parameters.GetString("network_id")
	if err != nil {
		return message.Fail(err.Error())
	}
	address, err := request.Parameters.GetString("address")
	if err != nil {
		return message.Fail(err.Error())
	}

	if smartcontract.ExistInDatabase(db, network_id, address) {
		return message.Fail("Smartcontract not registered in the database")
	}

	s, err := smartcontract.GetFromDatabase(db, network_id, address)
	if err != nil {
		return message.Fail("Failed to get smartcontract from database: " + err.Error())
	}

	return message.Reply{
		Status:     "OK",
		Message:    "",
		Parameters: key_value.Empty().Set("smartcontract", s),
	}
}
