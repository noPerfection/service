/*Categorized log containing log name and output parameters*/
package categorizer

import (
	"github.com/blocklords/gosds/message"
	"github.com/blocklords/gosds/remote"
	spaghetti_log "github.com/blocklords/gosds/spaghetti/log"
)

// The Smartcontract Event Log
type Log struct {
	ID             uint64 // ID in the database
	NetworkId      string // Network ID
	Txid           string // Transaction ID where it occured
	BlockNumber    uint64
	BlockTimestamp uint64
	LogIndex       uint                   // Log index in the block
	Address        string                 // Smartcontract address
	Log            string                 // Event log name
	Output         map[string]interface{} // Event log parameters
}

// Call categorizer.NewLog().AddMetadata().AddSmartcontractData()
// DON'T call it as a single function
func NewLog(log string, output map[string]interface{}) *Log {
	return &Log{
		Log:    log,
		Output: output,
	}
}

// Add the metadata such as transaction id and log index from spaghetti data
func (log *Log) AddMetadata(spaghetti_log *spaghetti_log.Log) *Log {
	log.Txid = spaghetti_log.Txid
	log.BlockNumber = spaghetti_log.BlockNumber
	log.BlockTimestamp = spaghetti_log.BlockTimestamp
	log.LogIndex = spaghetti_log.LogIndex
	return log
}

// add the smartcontract to which this log belongs too using categorizer.Smartcontract
func (log *Log) AddSmartcontractData(smartcontract *Smartcontract) *Log {
	log.NetworkId = smartcontract.NetworkId
	log.Address = smartcontract.Address
	return log
}

// Convert to the Map[string]interface
func (log *Log) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"network_id":      log.NetworkId,
		"txid":            log.Txid,
		"block_timestamp": log.BlockTimestamp,
		"block_number":    log.BlockNumber,
		"log_index":       log.LogIndex,
		"address":         log.Address,
		"log":             log.Log,
		"output":          log.Output,
	}
}

// Creates a new Log from the json object
func ParseLog(blob map[string]interface{}) (*Log, error) {
	network_id, err := message.GetString(blob, "network_id")
	if err != nil {
		return nil, err
	}
	address, err := message.GetString(blob, "address")
	if err != nil {
		return nil, err
	}
	txid, err := message.GetString(blob, "txid")
	if err != nil {
		return nil, err
	}
	log_index, err := message.GetUint64(blob, "log_index")
	if err != nil {
		return nil, err
	}

	block_timestamp, err := message.GetUint64(blob, "block_timestamp")
	if err != nil {
		return nil, err
	}
	block_number, err := message.GetUint64(blob, "block_number")
	if err != nil {
		return nil, err
	}

	log_name, err := message.GetString(blob, "log")
	if err != nil {
		return nil, err
	}

	output, err := message.GetMap(blob, "output")
	if err != nil {
		return nil, err
	}

	log := Log{
		NetworkId:      network_id,
		Txid:           txid,
		BlockNumber:    block_number,
		BlockTimestamp: block_timestamp,
		LogIndex:       uint(log_index),
		Address:        address,
		Log:            log_name,
		Output:         output,
	}

	return &log, nil
}

// Return list of logs for the transaction keys from the remote SDS Categorizer.
// For the transaction keys see
// github.com/blocklords/gosds/categorizer/transaction.go TransactionKey()
func RemoteLogs(socket *remote.Socket, keys []string) ([]*Log, error) {
	request := message.Request{
		Command: "log_get_all",
		Parameters: map[string]interface{}{
			"keys": keys,
		},
	}
	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return nil, err
	}

	raw_logs, err := message.GetMapList(params, "logs")
	if err != nil {
		return nil, err
	}

	logs := make([]*Log, len(raw_logs))
	for i, raw := range raw_logs {
		log, err := ParseLog(raw)
		if err != nil {
			return nil, err
		}
		logs[i] = log
	}

	return logs, nil
}

// Parse the raw event data using SDS Log.
// parsing events using JSON abi is harder in golang, therefore we use javascript
// implementation called SDS Log.
func RemoteLogParse(socket *remote.Socket, network_id string, address string, data string, topics []string) (string, map[string]interface{}, error) {
	request := message.Request{
		Command: "parse",
		Parameters: map[string]interface{}{
			"network_id": network_id,
			"address":    address,
			"data":       data,
			"topics":     topics,
		},
	}

	params, err := socket.RequestRemoteService(&request)
	if err != nil {
		return "", nil, err
	}

	name, err := message.GetString(params, "name")
	if err != nil {
		return "", nil, err
	}
	args, err := message.GetMap(params, "args")
	if err != nil {
		return "", nil, err
	}

	return name, args, nil
}
