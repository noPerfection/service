package blockchain

import (
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
)

type Number uint64
type Timestamp uint64

type Block struct {
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
func NewFromKeyValueParameter(parameters key_value.KeyValue) (Block, error) {
	var block Block
	err := parameters.ToInterface(&block)
	if err != nil {
		return block, fmt.Errorf("failed to convert key-value of Configuration to interface %v", err)
	}

	return block, nil
}

// Extracts the block timestamp from the key value map
func NewNumberFromKeyValueParameter(parameters key_value.KeyValue) (Number, error) {
	number, err := parameters.GetUint64("block_number")
	if err != nil {
		return 0, fmt.Errorf("parameter.GetUint64: %w", err)
	}

	return Number(number), nil
}

// Extracts the block timestamp from the key value map
func NewTimestampFromKeyValueParameter(parameters key_value.KeyValue) (Timestamp, error) {
	block_timestamp, err := parameters.GetUint64("block_timestamp")
	if err != nil {
		return 0, fmt.Errorf("parameter.GetUint64: %w", err)
	}

	return Timestamp(block_timestamp), nil
}

func New(number uint64, timestmap uint64) Block {
	return Block{
		Number:    Number(number),
		Timestamp: Timestamp(timestmap),
	}
}

func NewTimestamp(v uint64) Timestamp {
	return Timestamp(v)
}

func NewNumber(v uint64) Number {
	return Number(v)
}
