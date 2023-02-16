// The new.go keeps the functions that creates a new Abi from given parameters
package abi

import (
	"encoding/json"
	"fmt"
)

// creates the Abi data based on the abi JSON. The function calculates the abi hash
// but won't set it in the database.
func New(body interface{}) (*Abi, error) {
	abi := Abi{Body: body, exists: false}
	byt, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	abi.Bytes = byt
	abi.CalculateAbiHash()

	return &abi, nil
}

// creates the Abi data based on the JSON string. This function calculates the abi hash
// but won't set it in the database.
func FromBytes(bytes []byte) *Abi {
	body := []interface{}{}
	err := json.Unmarshal(bytes, &body)
	if err != nil {
		fmt.Println("Failed to convert abi bytes to body interface")
	}

	abi := Abi{Body: body, exists: false, Bytes: bytes}
	return &abi
}
