package data_type

import (
	"encoding/json"
)

// Wraps the JSON abi interface to the internal data type.
// It's blockchain agnostic.
func Serialize(body interface{}) ([]byte, error) {
	bytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func Deserialize(bytes []byte, body interface{}) error {
	err := json.Unmarshal(bytes, &body)

	return err
}
