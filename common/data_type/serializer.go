package data_type

import (
	"encoding/json"
	"fmt"
)

// Wraps the JSON abi interface to the internal data type.
// It's blockchain agnostic.
func Serialize(body interface{}) ([]byte, error) {
	bytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("json: '%w'", err)
	}
	return bytes, nil
}

// Deserialize the given string to the map, slice
// or struct.
func Deserialize(bytes []byte, body interface{}) error {
	err := json.Unmarshal(bytes, body)

	if err != nil {
		return fmt.Errorf("json: '%w'", err)
	}

	return nil
}
