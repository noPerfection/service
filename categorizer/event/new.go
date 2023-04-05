package event

import (
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
)

// Call categorizer.NewLog().AddMetadata().AddSmartcontractData()
// DON'T call it as a single function otherwise
// there is no guarantee that event is valid
func New(event_name string, parameters key_value.KeyValue) *Log {
	return &Log{
		Name:       event_name,
		Parameters: parameters,
	}
}

// Creates a new Log from the json object
func NewFromMap(blob key_value.KeyValue) (*Log, error) {
	var log Log
	err := blob.ToInterface(&log)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize key-value %v", err)
	}

	return &log, nil
}
