// The new.go keeps the functions that creates a new Abi from given parameters
package abi

import (
	"fmt"

	"github.com/blocklords/sds/common/data_type"
	"github.com/blocklords/sds/common/data_type/key_value"
)

// Wraps the JSON abi interface to the internal data type.
// It's blockchain agnostic.
func New(kv key_value.KeyValue) (*Abi, error) {
	var abi Abi
	id, err := kv.GetString("id")
	if err != nil {
		return nil, fmt.Errorf("key_value.GetString(id): %w", err)
	}
	if len(id) == 0 {
		return nil, fmt.Errorf("missing `id` parameter")
	} else {
		abi.Id = id
	}
	bytes, err := kv.GetString("bytes")
	if err != nil {
		return nil, fmt.Errorf("key_value.GetString(bytes): %w", err)
	}

	if len(bytes) == 0 {
		return nil, fmt.Errorf("missing `bytes` parameter")
	}
	unprefixed := data_type.DecodeJsonPrefixed(bytes)
	if len(unprefixed) == 0 {
		return nil, fmt.Errorf("parameter `bytes` is not a json prefixed string")
	}
	abi.Bytes = []byte(unprefixed)

	if err := abi.format_bytes(); err != nil {
		return nil, fmt.Errorf("format_bytes: %w", err)
	}

	return &abi, nil
}

// The bytes data are given as a JSON
// It will generate ID.
func NewFromInterface(body interface{}) (*Abi, error) {
	bytes, err := data_type.Serialize(body)
	if err != nil {
		return nil, err
	}
	return NewFromBytes(bytes)
}

// creates the Abi data based on the JSON string. This function calculates the abi hash
// but won't set it in the database.
func NewFromBytes(bytes []byte) (*Abi, error) {
	abi := Abi{Bytes: bytes}
	err := abi.GenerateId()
	if err != nil {
		return nil, fmt.Errorf("GenerateId: %w", err)
	}

	return &abi, nil
}
