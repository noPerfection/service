package block

import (
	"fmt"
	"strings"

	spaghetti_log "github.com/blocklords/sds/blockchain/event"
	"github.com/blocklords/sds/blockchain/evm/event"
	"github.com/blocklords/sds/common/blockchain"

	eth_types "github.com/ethereum/go-ethereum/core/types"
)

type Block struct {
	NetworkId string
	Header    blockchain.BlockHeader
	RawLogs   []spaghetti_log.RawLog
}

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

// Returns the smartcontract information
// Todo Get the logs for the blockchain
// Rather than getting transactions
func (block *Block) GetForSmartcontract(address string) []spaghetti_log.RawLog {
	logs := make([]spaghetti_log.RawLog, 0)

	for _, log := range block.RawLogs {
		if strings.EqualFold(address, log.Transaction.SmartcontractKey.Address) {
			logs = append(logs, log)
		}
	}

	return logs
}
