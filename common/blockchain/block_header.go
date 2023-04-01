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

func (n Number) Increment() Number {
	return n + Number(1)
}

func (n Number) Value() uint64 {
	return uint64(n)
}

func (t Timestamp) Value() uint64 {
	return uint64(t)
}

// Extracts the block parameters from the given key value map
func NewHeaderFromKeyValueParameter(parameters key_value.KeyValue) (BlockHeader, error) {
	var block BlockHeader
	err := parameters.ToInterface(&block)
	if err != nil {
		return block, fmt.Errorf("failed to convert key-value of Configuration to interface %v", err)
	}

	if block.Number.Value() == 0 || block.Timestamp.Value() == 0 {
		return block, fmt.Errorf("one of the parameters is 0")
	}

	return block, nil
}

// Extracts the block timestamp from the key value map
func NewNumberFromKeyValueParameter(parameters key_value.KeyValue) (Number, error) {
	number, err := parameters.GetUint64("block_number")
	if err != nil {
		return 0, fmt.Errorf("parameter.GetUint64: %w", err)
	}

	if number == 0 {
		return 0, fmt.Errorf("parameter is 0")
	}

	return Number(number), nil
}

// Extracts the block timestamp from the key value map
func NewTimestampFromKeyValueParameter(parameters key_value.KeyValue) (Timestamp, error) {
	block_timestamp, err := parameters.GetUint64("block_timestamp")
	if err != nil {
		return 0, fmt.Errorf("parameter.GetUint64: %w", err)
	}

	if block_timestamp == 0 {
		return 0, fmt.Errorf("parameter is 0")
	}

	return Timestamp(block_timestamp), nil
}

func NewHeader(number uint64, timestmap uint64) BlockHeader {
	return BlockHeader{
		Number:    NewNumber(number),
		Timestamp: NewTimestamp(timestmap),
	}
}

func NewTimestamp(v uint64) Timestamp {
	return Timestamp(v)
}

func NewNumber(v uint64) Number {
	return Number(v)
}
