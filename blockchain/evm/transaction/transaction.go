package transaction

import (
	"encoding/hex"
	"fmt"

	"github.com/blocklords/gosds/blockchain/evm/util"

	"github.com/blocklords/gosds/blockchain/transaction"

	eth_types "github.com/ethereum/go-ethereum/core/types"
)

func New(network_id string, block_number uint64, transaction_index uint, tx *eth_types.Transaction) (*transaction.Transaction, error) {
	msg, err := tx.AsMessage(eth_types.LatestSignerForChainID(tx.ChainId()), tx.GasPrice())
	if err != nil {
		fmt.Println("Error parsing transaction. Failed to get 'From' field. Error: ", err)
		return nil, err
	}

	bigValue := util.WeiToEther(tx.Value())
	value, _ := bigValue.Float64()
	toAddr := tx.To()
	to := ""
	if toAddr != nil {
		to = toAddr.Hex()
	}

	return &transaction.Transaction{
		NetworkId:      network_id,
		BlockNumber:    block_number,
		BlockTimestamp: 0,
		Txid:           tx.Hash().String(),
		TxIndex:        transaction_index,
		TxFrom:         msg.From().Hex(),
		TxTo:           to,
		Data:           hex.EncodeToString(tx.Data()),
		Value:          value,
	}, nil
}
