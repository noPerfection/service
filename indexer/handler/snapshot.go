package handler

import (
	"github.com/blocklords/sds/indexer/event"
	"github.com/blocklords/sds/service/log"
	"github.com/blocklords/sds/service/remote"

	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/database"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/service/communication/command"
	"github.com/blocklords/sds/service/communication/message"
)

const SNAPSHOT_LIMIT = uint64(500)

// Snapshot is the parameters of the request.Message
// for getting categorized events from database
//
//   - BlockTimestamp logs are fetched atleast from this time
//   - SmartcontractKeys are the list of smartcontracts to filter logs for
//     SmartcontractKeys can't be empty.
type Snapshot struct {
	BlockTimestamp    blockchain.Timestamp    `json:"block_timestamp"`
	SmartcontractKeys []smartcontract_key.Key `json:"smartcontract_keys"`
}

// SnapshotReply is the parameters of the message.Reply
// for getting categorized events from database
//
//   - BlockTimestamp logs is the block timestamp of the recent log
//     If no logs were given, then it will return the latest categorized block number
//   - Logs are the list of logs
type SnapshotReply struct {
	BlockTimestamp blockchain.Timestamp `json:"block_timestamp"`
	Logs           []event.Log          `json:"logs"`
}

// Return the SNAPSHOT_LIMIT categorized logs since the block_timestamp
// that matches topic_filter
//
// This function is called by the SDK through SDS Gateway
func GetSnapshot(request message.Request, _ log.Logger, app_parameters ...interface{}) message.Reply {
	if len(app_parameters) < 1 {
		return message.Fail("missing database client socket in app parameters")
	}
	db_con := app_parameters[0].(*remote.ClientSocket)

	var snapshot Snapshot
	err := request.Parameters.ToInterface(&snapshot)
	if err != nil {
		return message.Fail("parameter parsing error: " + err.Error())
	}
	if len(snapshot.SmartcontractKeys) == 0 {
		return message.Fail("no smartcontract_keys")
	}

	if err := snapshot.BlockTimestamp.Validate(); err != nil {
		return message.Fail("snapshot.BlockTimestamp.Validate(): " + err.Error())
	}
	for _, key := range snapshot.SmartcontractKeys {
		if err := key.Validate(); err != nil {
			return message.Fail("snapshot.SmartcontractKeys.Validate(): " + err.Error())
		}
	}

	var crud database.Crud = &event.Log{}
	condition := key_value.Empty().
		Set("smartcontract_keys", snapshot.SmartcontractKeys).
		Set("block_timestamp", snapshot.BlockTimestamp).
		Set("limit", SNAPSHOT_LIMIT)

	var logs []event.Log
	err = crud.SelectAllByCondition(db_con, condition, &logs)
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
