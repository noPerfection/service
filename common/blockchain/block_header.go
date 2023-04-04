package blockchain

import (
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
)

type Number uint64
type Timestamp uint64

// Header includes only block number and block timestamp
// We don't keep the proof or merkle tree.
type BlockHeader struct {
	Number    Number    `json:"block_number"`
	Timestamp Timestamp `json:"block_timestamp"`
}

func (header *BlockHeader) Validate() error {
	if err := header.Number.Validate(); err != nil {
		return fmt.Errorf("Number.Validate: %w", err)
	}
	if err := header.Timestamp.Validate(); err != nil {
		return fmt.Errorf("Timestamp.Validate: %w", err)
	}
	return nil
}

func (n Number) Increment() Number {
	return n + Number(1)
}

func (n Number) Value() uint64 {
	return uint64(n)
}

func (n Number) Validate() error {
	if n.Value() == 0 {
		return fmt.Errorf("number is 0")
	}
	return nil
}

func (t Timestamp) Value() uint64 {
	return uint64(t)
}

func (t Timestamp) Validate() error {
	if t.Value() == 0 {
		return fmt.Errorf("timestamp is 0")
	}
	return nil
}

// Extracts the block parameters from the given key value map
func NewHeaderFromKeyValueParameter(parameters key_value.KeyValue) (BlockHeader, error) {
	var block BlockHeader
	err := parameters.ToInterface(&block)
	if err != nil {
		return block, fmt.Errorf("failed to convert key-value of Configuration to interface %v", err)
	}

	if err := block.Validate(); err != nil {
		return block, fmt.Errorf("Validate: %w", err)
	}

	return block, nil
}

// Extracts the block timestamp from the key value map
func NewNumberFromKeyValueParameter(parameters key_value.KeyValue) (Number, error) {
	number, err := parameters.GetUint64("block_number")
	if err != nil {
		return 0, fmt.Errorf("parameter.GetUint64: %w", err)
	}

	return NewNumber(number)
}

// Extracts the block timestamp from the key value map
func NewTimestampFromKeyValueParameter(parameters key_value.KeyValue) (Timestamp, error) {
	block_timestamp, err := parameters.GetUint64("block_timestamp")
	if err != nil {
		return 0, fmt.Errorf("parameter.GetUint64: %w", err)
	}

	return NewTimestamp(block_timestamp)
}

func NewHeader(number uint64, timestmap uint64) (BlockHeader, error) {
	header := BlockHeader{
		Number:    Number(number),
		Timestamp: Timestamp(timestmap),
	}
	if err := header.Validate(); err != nil {
		return BlockHeader{}, fmt.Errorf("Validate: %w", err)
	}

	return header, nil
}

func NewTimestamp(v uint64) (Timestamp, error) {
	n := Timestamp(v)
	if err := n.Validate(); err != nil {
		return 0, fmt.Errorf("Validate: %w", err)
	}
	return n, nil
}

func NewNumber(v uint64) (Number, error) {
	n := Number(v)
	if err := n.Validate(); err != nil {
		return 0, fmt.Errorf("Validate: %w", err)
	}
	return n, nil
}
