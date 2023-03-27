package event

import (
	"errors"
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
)

// Convert the JSON into spaghetti.Log
// https://docs.soliditylang.org/en/v0.8.4/abi-spec.html?highlight=anonymous#json
func New(parameters key_value.KeyValue) (*RawLog, error) {
	var log RawLog
	err := parameters.ToInterface(&log)
	if err != nil {
		return nil, fmt.Errorf("key-value convert: %w", err)
	}

	return &log, nil
}

// Parse list of Logs into array of spaghetti.Log
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
