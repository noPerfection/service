package log

import "github.com/blocklords/gosds/message"

// Call categorizer.NewLog().AddMetadata().AddSmartcontractData()
// DON'T call it as a single function
func New(log string, output map[string]interface{}) *Log {
	return &Log{
		Log:    log,
		Output: output,
	}
}

// Creates a new Log from the json object
func NewFromMap(blob map[string]interface{}) (*Log, error) {
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
