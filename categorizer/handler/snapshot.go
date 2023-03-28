package handler

import (
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/categorizer/event"
	"github.com/blocklords/sds/db"

	"github.com/blocklords/sds/app/command"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/smartcontract_key"
)

const SNAPSHOT_LIMIT = uint64(500)

type Snapshot struct {
	BlockTimestamp    blockchain.Timestamp    `json:"block_timestamp"`
	SmartcontractKeys []smartcontract_key.Key `json:"smartcontract_keys"`
}

type SnapshotReply struct {
	BlockTimestamp blockchain.Timestamp `json:"block_timestamp"`
	Logs           []event.Log          `json:"logs"`
}

// Return the SNAPSHOT_LIMIT categorized logs since the block_timestamp
// that matches topic_filter
//
// This function is called by the SDK through SDS Gateway
func GetSnapshot(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	db_con := parameters[0].(*db.Database)

	var snapshot Snapshot
	err := request.Parameters.ToInterface(&snapshot)
	if err != nil {
		return message.Fail("parameter parsing error: " + err.Error())
	}
	if len(snapshot.SmartcontractKeys) == 0 {
		return message.Fail("no smartcontract_keys")
	}

	logs, err := event.GetLogsFromDb(db_con, snapshot.SmartcontractKeys, snapshot.BlockTimestamp+1, SNAPSHOT_LIMIT)
	if err != nil {
		return message.Fail("database error to filter logs: " + err.Error())
	}

	recent_block_timestamp := snapshot.BlockTimestamp
	for _, log := range logs {
		if log.BlockHeader.Timestamp > recent_block_timestamp {
			recent_block_timestamp = log.BlockHeader.Timestamp
		}
	}

	reply_parameters := SnapshotReply{
		BlockTimestamp: recent_block_timestamp,
		Logs:           logs,
	}

	reply_message, err := command.Reply(&reply_parameters)

	if err != nil {
		return message.Fail("failed to reply: " + err.Error())
	}

	return reply_message
}
