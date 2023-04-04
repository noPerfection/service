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
	err := kv.ToInterface(&abi)

	if err != nil {
		return nil, fmt.Errorf("key_value.ToInterface(static/abi.Abi): %w", err)
	}

	if len(abi.Id) == 0 {
		return nil, fmt.Errorf("missing `id` parameter")
	}
	if len(abi.Bytes) == 0 {
		return nil, fmt.Errorf("missing `bytes` parameter")
	}
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
