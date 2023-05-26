package event

import (
	"errors"
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
)

// Convert the JSON into blockchain.Log
// https://docs.soliditylang.org/en/v0.8.4/abi-spec.html?highlight=anonymous#json
func New(parameters key_value.KeyValue) (*RawLog, error) {
	err := parameters.Exist("log_index")
	if err != nil {
		return nil, fmt.Errorf("validation of log_index: %w", err)
	}

	var log RawLog
	err = parameters.ToInterface(&log)
	if err != nil {
		return nil, fmt.Errorf("key-value convert: %w", err)
	}

	err = log.Transaction.Validate()
	if err != nil {
		return nil, fmt.Errorf("transaction.Validate: %w", err)
	}

	return &log, nil
}

// Parse list of Logs into array of blockchain.Log
func NewLogs(raw_logs []interface{}) ([]RawLog, error) {
	logs := make([]RawLog, len(raw_logs))
	for i, raw := range raw_logs {
		if raw == nil {
			continue
		}
		log_map, ok := raw.(map[string]interface{})
		if !ok {
			return nil, errors.New("the log is not a map")
		}
		l, err := New(log_map)
		if err != nil {
			return nil, fmt.Errorf("log[%d] converting to Log: %w", i, err)
		}
		logs[i] = *l
	}
	return logs, nil
}
