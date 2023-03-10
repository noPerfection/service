package transaction

import (
	"encoding/hex"
	"fmt"

	"github.com/blocklords/sds/blockchain/evm/util"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/static/smartcontract/key"

	"github.com/blocklords/sds/blockchain/transaction"

	eth_types "github.com/ethereum/go-ethereum/core/types"
)

func New(network_id string, block_number uint64, transaction_index uint, tx *eth_types.Transaction) (*transaction.RawTransaction, error) {
	msg, err := tx.AsMessage(eth_types.LatestSignerForChainID(tx.ChainId()), tx.GasPrice())
	if err != nil {
		return nil, fmt.Errorf("error parsing transaction. Failed to get 'From' field: %w", err)
	}

	bigValue := util.WeiToEther(tx.Value())
	value, _ := bigValue.Float64()
	toAddr := tx.To()
	to := ""
	if toAddr != nil {
		to = toAddr.Hex()
	}

	key := key.New(network_id, to)
	block := blockchain.NewBlock(block_number, 0)
	tx_key := blockchain.TransactionKey{
		Id:    tx.Hash().String(),
		Index: transaction_index,
	}

	return &transaction.RawTransaction{
		Key:            key,
		Block:          block,
		TransactionKey: tx_key,
		From:           msg.From().Hex(),
		Data:           hex.EncodeToString(tx.Data()),
		Value:          value,
	}, nil
}
