// The EVM blockchain client
// Any reply from client is validated.
// Then the reply is converted into the internal data type.
package client

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/blocklords/gosds/blockchain/evm/transaction"
	"github.com/blocklords/gosds/blockchain/network/provider"
	spaghetti_transaction "github.com/blocklords/gosds/blockchain/transaction"

	"github.com/ethereum/go-ethereum"

	eth_common "github.com/ethereum/go-ethereum/common"
	eth_types "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// todo any call should be with a context and repititon
type Client struct {
	network_id string
	client     *ethclient.Client
	ctx        context.Context
	provider   provider.Provider
}

// Create a network client connected to the blockchain based on a Static parameters
// Static parameters include the node url
func new(p provider.Provider) (*Client, error) {
	provider_url := p.Url

	ctx := context.TODO()
	client, err := ethclient.DialContext(ctx, provider_url)
	if err != nil {
		return nil, fmt.Errorf(`failed to connect to blockchain. please try again later: %w`, err)
	}

	return &Client{
		client:   client,
		ctx:      ctx,
		provider: p,
	}, nil
}

// Creates a network clients connected to the blockchain network for each static parameter
func new_clients(providers []provider.Provider) ([]*Client, error) {
	network_clients := make([]*Client, len(providers))

	for i, p := range providers {
		new_client, err := new(p)
		if err != nil {
			return nil, fmt.Errorf("New client[%d]: %w", i, err)
		}

		network_clients[i] = new_client
	}

	return network_clients, nil
}

//////////////////////////////////////////////////////////
//
// Blockchain related functions
//
/////////////////////////////////////////////////////////

// Returns the block timestamp from the blockchain
func (c *Client) GetBlockTimestamp(block_number uint64) (uint64, error) {
	header, err := c.client.HeaderByNumber(c.ctx, big.NewInt(int64(block_number)))
	if err != nil {
		return 0, errors.New("failed to fetch block information from provider: " + err.Error())
	}

	return header.Time, nil
}

// Returns the most recent block number from blockchain
func (c *Client) GetRecentBlockNumber() (uint64, error) {
	block_number, err := c.client.BlockNumber(c.ctx)
	if err != nil {
		return 0, fmt.Errorf("provider block number: %w", err)
	}

	return block_number, nil
}

// Returns the information about the specific transaction from the blockchain
// The transaction is converted into the gosds/spaghetti/transaction.Transaction data type
func (c *Client) GetTransaction(transaction_id string) (*spaghetti_transaction.Transaction, error) {
	transaction_hash := eth_common.HexToHash(transaction_id)
	var transaction_raw *eth_types.Transaction
	var pending bool
	var err error

	attempt := 10
	for {
		transaction_raw, pending, err = c.client.TransactionByHash(c.ctx, transaction_hash)
		if pending {
			return nil, fmt.Errorf("the transaction is in the pending mode. please try again later fetching %s", transaction_hash)
		}
		if err == nil {
			break
		}
		if attempt == 0 {
			return nil, fmt.Errorf("transaction by hash error after 10 attempts: " + err.Error())
		}
		fmt.Printf("client.TransactionByHash txid (%s) attempts left=%d: %s", transaction_hash, attempt, err.Error())
		time.Sleep(time.Second * 1)
		attempt--
	}

	var receipt *eth_types.Receipt
	attempt = 10
	for {
		receipt, err = c.client.TransactionReceipt(c.ctx, transaction_hash)
		if err == nil {
			break
		}
		if attempt == 0 {
			return nil, fmt.Errorf("transaction receipt error after 10 attempts: " + err.Error())
		}
		fmt.Printf("client.TransactionReceipt txid (%s) attempts left=%d: %s", transaction_hash, attempt, err.Error())
		time.Sleep(time.Second * 1)
		attempt--
	}

	tx, parse_err := transaction.New(c.network_id, receipt.BlockNumber.Uint64(), receipt.TransactionIndex, transaction_raw)
	if parse_err != nil {
		return nil, fmt.Errorf("transaction.New: %w", parse_err)
	}
	if tx.TxTo == "" {
		tx.TxTo = receipt.ContractAddress.Hex()
	}

	return tx, nil
}

// Returns the block logs
func (c *Client) GetBlockLogs(block_number uint64) ([]eth_types.Log, error) {
	big_int := big.NewInt(int64(block_number))

	query := ethereum.FilterQuery{
		FromBlock: big_int,
		ToBlock:   big_int,
		Addresses: []eth_common.Address{},
	}

	raw_logs, log_err := c.filter_logs(query)
	if log_err != nil {
		return nil, fmt.Errorf("client.filter_logs for block number %d: %w", block_number, log_err)
	}
	return raw_logs, nil
}

// Returns the logs for a block range
func (c *Client) GetBlockRangeLogs(block_number_from uint64, block_number_to uint64, addresses []string) ([]eth_types.Log, error) {
	big_from := big.NewInt(int64(block_number_from))
	big_to := big.NewInt(int64(block_number_to))

	eth_addresses := make([]eth_common.Address, len(addresses))
	for i, address := range addresses {
		eth_address := eth_common.HexToAddress(address)
		eth_addresses[i] = eth_address
	}

	query := ethereum.FilterQuery{
		FromBlock: big_from,
		ToBlock:   big_to,
		Addresses: eth_addresses,
	}

	raw_logs, log_err := c.filter_logs(query)
	if log_err != nil {
		return nil, fmt.Errorf("client.filter_logs for between %d - %d, addresses amount (%d): %w", block_number_from, block_number_to, len(addresses), log_err)
	}
	return raw_logs, log_err
}

func (c *Client) filter_logs(query ethereum.FilterQuery) ([]eth_types.Log, error) {
	var raw_logs []eth_types.Log
	var log_err error
	attempt := 5
	for {
		raw_logs, log_err = c.client.FilterLogs(c.ctx, query)
		if log_err == nil {
			break
		}
		time.Sleep(10 * time.Second)
		attempt--
		if attempt == 0 {
			return nil, fmt.Errorf("failed to get the logs in 5 attempts. network id: %s, query %v: %w", c.provider.Url, query, log_err)
		}
	}

	return raw_logs, nil
}
