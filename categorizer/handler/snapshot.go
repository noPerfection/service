package handler

import (
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/categorizer/event"
	"github.com/blocklords/sds/db"

	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
)

const SNAPSHOT_LIMIT = uint64(500)

// Return the SNAPSHOT_LIMIT categorized logs since the block_timestamp
// that matches topic_filter
//
// This function is called by the SDK through SDS Gateway
func GetSnapshot(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	db_con := parameters[0].(*db.Database)

	block_timestamp_from, err := blockchain.NewTimestampFromKeyValueParameter(request.Parameters)
	if err != nil {
		return message.Fail(err.Error())
	}
	raw_smartcontract_keys, err := request.Parameters.GetKeyValueList("smartcontract_keys")
	if err != nil {
		return message.Fail("GetKeyValueList: " + err.Error())
	}
	if len(raw_smartcontract_keys) == 0 {
		return message.Fail("no smartcontract_keys")
	}
	smartcontract_keys := make([]smartcontract_key.Key, len(raw_smartcontract_keys))
	for i, raw_key := range raw_smartcontract_keys {
		var key smartcontract_key.Key
		err := raw_key.ToInterface(&key)
		if err != nil {
			return message.Fail("failed to decode smartcontract key: " + err.Error())
		}
		smartcontract_keys[i] = key
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
			"logs":            logs,
			"block_timestamp": block_timestamp_to,
		}),
	}

	return reply
}
