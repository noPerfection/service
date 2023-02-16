package block

import (
	"fmt"
	"strings"

	"github.com/blocklords/gosds/spaghetti/log"
	"github.com/blocklords/gosds/spaghetti/transaction"

	eth_types "github.com/ethereum/go-ethereum/core/types"
)

type Block struct {
	NetworkId      string
	BlockNumber    uint64
	BlockTimestamp uint64
	Transactions   []*transaction.Transaction
	Logs           []*log.Log
}

func SetTransactions(block *Block, raw_transactions []*eth_types.Transaction) error {
	transactions := make([]*transaction.Transaction, len(raw_transactions))

	for txIndex, rawTx := range raw_transactions {
		tx, txErr := transaction.New(block.NetworkId, block.BlockNumber, uint(txIndex), rawTx)
		if txErr != nil {
			return fmt.Errorf("failed to set the block transactions. transaction parse error: %v", txErr)
		}

		transactions[txIndex] = tx
	}

	block.Transactions = transactions
	return nil
}

func SetLogs(block *Block, raw_logs []eth_types.Log) error {
	var logs []*log.Log
	for _, rawLog := range raw_logs {
		if rawLog.Removed {
			continue
		}

		log, txErr := log.NewFromRawLog(block.NetworkId, &rawLog)
		if txErr != nil {
			return txErr
		}

		logs = append(logs, log)
	}

	block.Logs = logs

	return nil
}

// Returns the smartcontract information
func (block *Block) GetForSmartcontract(address string) ([]*transaction.Transaction, []*log.Log) {
	transactios := make([]*transaction.Transaction, 0)
	logs := make([]*log.Log, 0)

	for _, transaction := range block.Transactions {
		if strings.EqualFold(transaction.TxTo, address) {
			transactios = append(transactios, transaction)

			for _, log := range block.Logs {
				if strings.EqualFold(transaction.Txid, log.Txid) {
					logs = append(logs, log)
				}
			}
		}
	}

	return transactios, logs
}
