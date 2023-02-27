// The new.go keeps the functions that creates a new Abi from given parameters
package abi

import (
	"github.com/blocklords/gosds/common/data_type"
)

// Wraps the JSON abi interface to the internal data type.
// It's blockchain agnostic.
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
	abi := Abi{bytes: bytes}
	abi.GenerateId()

	return &abi, nil
}
