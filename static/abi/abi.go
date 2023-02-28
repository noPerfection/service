package abi

import (
	"encoding/base64"
	"errors"

	"github.com/blocklords/gosds/common/data_type"
)

type Abi struct {
	Bytes []byte `json:"bytes"`
	Id    string `json:"id"`
}

// Returns the abi content as a string
func (abi *Abi) ToString() string {
	return string(abi.Bytes)
}

// Creates the abi hash from the abi body
// The abi hash is the unique identifier of the abi
func (abi *Abi) GenerateId() {
	encoded := base64.StdEncoding.EncodeToString(abi.Bytes)
	abi.Id = encoded[0:8]
}

// Get the interface from the bytes
func (abi *Abi) Interface(body interface{}) error {
	err := data_type.Deserialize(abi.Bytes, &body)
	if err != nil {
		return errors.New("failed to convert abi bytes to body interface")
	}

	return nil
}
