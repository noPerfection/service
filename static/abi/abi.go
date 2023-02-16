package abi

import (
	"github.com/ethereum/go-ethereum/crypto"
)

type Abi struct {
	Bytes []byte
	// Body abi.ABI
	Body    interface{}
	AbiHash string
	exists  bool
}

// Creates the JSON object with abi hash and abi body.
func (abi *Abi) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"abi":      abi.Body,
		"abi_hash": abi.AbiHash,
	}
}

// Creates the abi hash from the abi body
// The abi hash is the unique identifier of the abi
func (a *Abi) CalculateAbiHash() {
	hash := crypto.Keccak256Hash(a.Bytes)
	a.AbiHash = hash.String()[2:10]
}

// check whether the abi when its build was built from the database or in memory
func (a *Abi) Exists() bool {
	return a.exists
}

// set the exists flag
func (a *Abi) SetExists(exists bool) {
	a.exists = exists
}
