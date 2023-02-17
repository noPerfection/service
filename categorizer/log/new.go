package log

import (
	"fmt"

	"github.com/blocklords/gosds/common/data_type/key_value"
)

// Call categorizer.NewLog().AddMetadata().AddSmartcontractData()
// DON'T call it as a single function
func New(log string, output map[string]interface{}) *Log {
	return &Log{
		Log:    log,
		Output: output,
	}
}

// Creates a new Log from the json object
func NewFromMap(blob key_value.KeyValue) (*Log, error) {
	i, err := blob.ToInterface()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize key-value %v", err)
	}

	log, ok := i.(Log)
	if !ok {
		return nil, fmt.Errorf("failed to convert intermediate interface to categorizer.Log %v", i)
	}

	return &log, nil
}
