package abi

import (
	"github.com/ethereum/go-ethereum/crypto"
)

type Abi struct {
	bytes []byte
	// Body abi.ABI
	Body    interface{} `json:"abi"`
	AbiHash string      `json:"abi_hash"`
	exists  bool
}

// Returns the abi content in string format
func (abi *Abi) ToString() string {
	return string(abi.bytes)
}

// Creates the abi hash from the abi body
// The abi hash is the unique identifier of the abi
func (a *Abi) CalculateAbiHash() {
	hash := crypto.Keccak256Hash(a.bytes)
	a.AbiHash = hash.String()[2:10]
}

// check whether the abi when its build was built from the database or in memory
func (a *Abi) Exists() bool {
	return a.exists
}

func (a *Abi) SetBytes(bytes []byte) {
	a.bytes = bytes
}

// set the exists flag
func (a *Abi) SetExists(exists bool) {
	a.exists = exists
}
