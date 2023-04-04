package transaction

import (
	"encoding/hex"
	"fmt"

	"github.com/blocklords/sds/blockchain/evm/util"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/smartcontract_key"

	"github.com/blocklords/sds/blockchain/transaction"

	eth_types "github.com/ethereum/go-ethereum/core/types"
)

func New(network_id string, block blockchain.BlockHeader, transaction_index uint, tx *eth_types.Transaction) (*transaction.RawTransaction, error) {
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

	key, err := smartcontract_key.New(network_id, to)
	if err != nil {
		return nil, fmt.Errorf("smartcontract_key.New: %w", err)
	}
	tx_key := blockchain.TransactionKey{
		Id:    tx.Hash().String(),
		Index: transaction_index,
	}

	return &transaction.RawTransaction{
		SmartcontractKey: key,
		BlockHeader:      block,
		TransactionKey:   tx_key,
		From:             msg.From().Hex(),
		Data:             hex.EncodeToString(tx.Data()),
		Value:            value,
	}, nil
}
