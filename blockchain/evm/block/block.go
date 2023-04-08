// Package block defines the collection of the smartcontract event logs
// for the given block or block range.
//
// If the raw logs are given for the block range, then the [Block.Header] will
// keep the values of the most recent raw log.
package block

import (
	"fmt"
	"strings"

	spaghetti_log "github.com/blocklords/sds/blockchain/event"
	"github.com/blocklords/sds/blockchain/evm/event"
	"github.com/blocklords/sds/common/blockchain"

	eth_types "github.com/ethereum/go-ethereum/core/types"
)

// EVM Block with the raw logs.
//
// It doesn't include the transactions.
// Since transactions are not supported by SDS.
type Block struct {
	// SDS network id to which this
	// block belongs
	NetworkId string
	// Block number and timestamp
	Header blockchain.BlockHeader
	// List of event logs
	RawLogs []spaghetti_log.RawLog
}

// Converts ethereum logs into SDS raw log.
// Then sets them as block logs.
// If the block has already logs, then
// they will be overwritten.
//
// It doesn't check for duplicate logs.
//
// Returns an error if failed to convert
// ethereum log to SDS raw log.
func SetLogs(block *Block, raw_logs []eth_types.Log) error {
	var logs []spaghetti_log.RawLog
	for _, rawLog := range raw_logs {
		if rawLog.Removed {
			continue
		}

		log, err := event.NewSpaghettiLog(block.NetworkId, block.Header.Timestamp, &rawLog)
		if err != nil {
			return fmt.Errorf("event.NewSpaghettiLog: %w", err)
		}
		logs = append(logs, *log)
	}

	block.RawLogs = logs

	return nil
}

// Returns the raw logs from block
// that are sent to the address.
func (block *Block) GetForSmartcontract(address string) []spaghetti_log.RawLog {
	logs := make([]spaghetti_log.RawLog, 0)

	for _, log := range block.RawLogs {
		if strings.EqualFold(address, log.Transaction.SmartcontractKey.Address) {
			logs = append(logs, log)
		}
	}

	return logs
}
