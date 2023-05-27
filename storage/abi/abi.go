// Package abi defines the abi of the smartcontract
//
// The db.go contains the database related functions of the ABI
package abi

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/blocklords/sds/common/data_type"
)

type Abi struct {
	Bytes []byte `json:"bytes"`
	Id    string `json:"id"`
}

// Returns the abi content as a string.
// The abi bytes are first formatted.
// If the abi parameters are invalid, then
// the ToString() returns empty string.
func (abi *Abi) ToString() string {
	if err := abi.format_bytes(); err != nil {
		return ""
	}
	return string(abi.Bytes)
}

// Creates the abi hash from the abi body
// The Abi ID is the unique identifier of the abi
//
// The Abi ID is the first 8 characters of the
// sha256 checksum
// representation of the abi.
//
// If the bytes field is invalid, then the id will be empty
func (abi *Abi) GenerateId() error {
	abi.Id = ""

	// re-serialize to remove the empty spaces
	if err := abi.format_bytes(); err != nil {
		return fmt.Errorf("format_bytes: %w", err)
	}
	encoded := sha256.Sum256(abi.Bytes)
	abi.Id = hex.EncodeToString(encoded[0:8])

	return nil
}

func (abi *Abi) format_bytes() error {
	// re-serialize to remove the empty spaces
	var json interface{}
	err := abi.Interface(&json)
	if err != nil {
		return fmt.Errorf("failed to deserialize: %w", err)
	}
	bytes, err := data_type.Serialize(json)
	if err != nil {
		return fmt.Errorf("failed to re-serialize: %w", err)
	}
	abi.Bytes = bytes

	return nil
}

// Get the interface from the bytes
// It converts the bytes into the JSON value
func (abi *Abi) Interface(body interface{}) error {
	err := data_type.Deserialize(abi.Bytes, body)
	if err != nil {
		return fmt.Errorf("data_type.Deserialize: %w", err)
	}

	return nil
}
