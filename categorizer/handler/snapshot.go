package handler

import (
	"github.com/charmbracelet/log"

	"github.com/blocklords/sds/categorizer/event"
	"github.com/blocklords/sds/db"
	"github.com/blocklords/sds/static/configuration"
	"github.com/blocklords/sds/static/smartcontract"

	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/topic"
)

const SNAPSHOT_LIMIT = uint64(500)

// Return the categorized logs of the SNAPSHOT_LIMIT amount since the block_timestamp_from
// For the topic_filter
//
// This function is called by the Gateway
func GetSnapshot(request message.Request, logger log.Logger, parameters ...interface{}) message.Reply {
	db_con := parameters[0].(*db.Database)

	/////////////////////////////////////////////////////////////////////////////
	//
	// Extract the parameters
	//
	/////////////////////////////////////////////////////////////////////////////
	block_timestamp_from, err := blockchain.NewTimestampFromKeyValueParameter(request.Parameters)
	if err != nil {
		return message.Fail(err.Error())
	}
	topic_filter, err := topic.NewFromKeyValueParameter(request.Parameters)
	if err != nil {
		return message.Fail("topic.NewFromKeyValueParameter: " + err.Error())
	}

	query, query_parameters := configuration.QueryFilterSmartcontract(topic_filter)

	smartcontract_keys, _, err := smartcontract.FilterKeysFromDatabase(db_con, query, query_parameters)
	if err != nil {
		return message.Fail("failed to filter smartcontracts by the topic filter:" + err.Error())
	} else if len(smartcontract_keys) == 0 {
		return message.Fail("no matching smartcontracts for the topic filter " + topic_filter.ToString())
	}

	logs, err := event.GetLogsFromDb(db_con, smartcontract_keys, block_timestamp_from, SNAPSHOT_LIMIT)
	if err != nil {
		return message.Fail("database error to filter logs: " + err.Error())
	}

	block_timestamp_to := block_timestamp_from
	for _, log := range logs {
		if log.BlockHeader.Timestamp > block_timestamp_to {
			block_timestamp_to = log.BlockHeader.Timestamp
		}
	}

	reply := message.Reply{
		Status: "OK",
		Parameters: key_value.New(map[string]interface{}{
			"logs":            data_type.ToMapList(logs),
			"block_timestamp": block_timestamp_to,
		}),
	}

	return reply
}
