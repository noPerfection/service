// Package client defines the RPC client that's connected to the Blockchain network.
//
// The client is the wrapper around [github.com/blocklords/sds/blockchain/network/provider]
// with the rating.
package client

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/remote/parameter"
	"github.com/blocklords/sds/blockchain/evm/transaction"
	"github.com/blocklords/sds/blockchain/network/provider"
	spaghetti_transaction "github.com/blocklords/sds/blockchain/transaction"
	"github.com/blocklords/sds/common/blockchain"

	"github.com/ethereum/go-ethereum"

	eth_common "github.com/ethereum/go-ethereum/common"
	eth_types "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	DECREASE_VALUE = 0.1    // The weight to decrease from the rating of the client if provider connection failed
	INCREASE_VALUE = 0.1    // The weight to increase to the rating of the client if the provider connection was successful
	DEFAULT_RATING = 100.0  // Initial rating
	MAX_RATING_CAP = 1000.0 // Increase the rating up until to this level
	MIN_RATING_CAP = 0.0    // Decrease the rating till this value
	STABLE_RATING  = 50.0   // Defines the rating weight when the client is considered unstable and won't be used anymore.
)

// Client is the wrapper around ethereum RPC client with the rating.
// Don't use this directly, rather use it with the [client.Manager]
//
// The manager will use the stable clients automatically.
type Client struct {
	client   *ethclient.Client
	ctx      context.Context
	provider provider.Provider
	Rating   float64
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
		Rating:   DEFAULT_RATING,
	}, nil
}

// Creates a network clients connected to the blockchain network for each static parameter
func new_clients(providers []provider.Provider) ([]*Client, error) {
	network_clients := make([]*Client, len(providers))

	for i, p := range providers {
		new_client, err := new(p)
		if err != nil {
			return nil, fmt.Errorf("new client[%d]: %w", i, err)
		}

		network_clients[i] = new_client
	}

	return network_clients, nil
}

// Increase the rating, means the provider url is stable
func (c *Client) increase_rating() {
	if c.Rating < MAX_RATING_CAP {
		c.Rating += INCREASE_VALUE
	}
}

// Decrease the rating, means the provider url is unstable
func (c *Client) decrease_rating() {
	if c.Rating > MIN_RATING_CAP {
		c.Rating -= INCREASE_VALUE
	}
}

// GetBlockTimestamp returns the timestamp of the block number from
// remote blockchain node
func (c *Client) GetBlockTimestamp(block_number uint64, app_config *configuration.Config) (uint64, error) {
	request_timeout := parameter.RequestTimeout(app_config)

	ctx, cancel := context.WithTimeout(c.ctx, request_timeout)
	defer cancel()

	header, err := c.client.HeaderByNumber(ctx, big.NewInt(int64(block_number)))
	if err != nil {
		c.decrease_rating()
		return 0, errors.New("failed to fetch block information from provider: " + err.Error())
	}

	c.increase_rating()
	return header.Time, nil
}

// GetRecentBlockNumber returns the current block number of the EVM blockchain from
// remote blockchain node.
func (c *Client) GetRecentBlockNumber(app_config *configuration.Config) (uint64, error) {
	request_timeout := parameter.RequestTimeout(app_config)

	ctx, cancel := context.WithTimeout(c.ctx, request_timeout)
	defer cancel()

	block_number, err := c.client.BlockNumber(ctx)
	if err != nil {
		c.decrease_rating()
		return 0, fmt.Errorf("provider block number: %w", err)
	}

	c.increase_rating()
	return block_number, nil
}

// GetTransaction returns the transaction parameters from the remote blockchain node.
//
// Example of calling is when a new Smartcontract is registered on SDS, using this function
// we get the information about the deployment, such as the first block number from which
// we begin the smartcontract categorization.
func (c *Client) GetTransaction(transaction_id string, app_config *configuration.Config) (*spaghetti_transaction.RawTransaction, error) {
	request_timeout := parameter.RequestTimeout(app_config)

	ctx, cancel := context.WithTimeout(c.ctx, request_timeout)
	defer cancel()

	transaction_hash := eth_common.HexToHash(transaction_id)

	transaction_raw, pending, err := c.client.TransactionByHash(ctx, transaction_hash)
	if pending {
		c.increase_rating()
		return nil, fmt.Errorf("the transaction is in the pending mode. please try again later fetching %s", transaction_hash)
	}
	if err != nil {
		c.decrease_rating()
		return nil, fmt.Errorf("client.TransactionByHash txid (%s): %w", transaction_hash, err)
	}
	c.increase_rating()

	new_ctx, new_cancel := context.WithTimeout(c.ctx, request_timeout)
	defer new_cancel()

	receipt, err := c.client.TransactionReceipt(new_ctx, transaction_hash)
	if err != nil {
		c.decrease_rating()
		return nil, fmt.Errorf("client.TransactionReceipt txid(%s): %w", transaction_hash, err)
	}
	c.increase_rating()

	block := blockchain.BlockHeader{
		Number: blockchain.Number(receipt.BlockNumber.Uint64()),
	}

	time_ctx, time_cancel := context.WithTimeout(c.ctx, request_timeout)
	defer time_cancel()
	block_raw, err := c.client.BlockByNumber(time_ctx, receipt.BlockNumber)
	if err != nil {
		c.decrease_rating()
		return nil, fmt.Errorf("client.BlockByNumber (%d): %w", block.Number, err)
	}
	c.increase_rating()
	block.Timestamp, err = blockchain.NewTimestamp(block_raw.Header().Time)
	if err != nil {
		return nil, fmt.Errorf("blockchain.NewTimestamp (%d): %w", block.Number, err)
	}

	tx, parse_err := transaction.New("", block, receipt.TransactionIndex, transaction_raw)
	if parse_err != nil {
		return nil, fmt.Errorf("transaction.New: %w", parse_err)
	}
	if tx.SmartcontractKey.Address == "" {
		tx.SmartcontractKey.Address = receipt.ContractAddress.Hex()
	}

	return tx, nil
}

// GetBlockLogs returns the smartcontract logs in the block number.
func (c *Client) GetBlockLogs(block_number uint64, app_config *configuration.Config) ([]eth_types.Log, error) {
	big_int := big.NewInt(int64(block_number))

	query := ethereum.FilterQuery{
		FromBlock: big_int,
		ToBlock:   big_int,
		Addresses: []eth_common.Address{},
	}

	request_timeout := parameter.RequestTimeout(app_config)

	raw_logs, log_err := c.filter_logs(query, request_timeout)
	if log_err != nil {
		return nil, fmt.Errorf("client.filter_logs for block number %d: %w", block_number, log_err)
	}
	return raw_logs, nil
}

// GetBlockRangeLogs returns the smartcontract logs between the block number range.
func (c *Client) GetBlockRangeLogs(block_number_from uint64, block_number_to uint64, addresses []string, app_config *configuration.Config) ([]eth_types.Log, error) {
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

	request_timeout := parameter.RequestTimeout(app_config)

	raw_logs, log_err := c.filter_logs(query, request_timeout)
	if log_err != nil {
		return nil, fmt.Errorf("client.filter_logs for between %d - %d, addresses amount (%d): %w", block_number_from, block_number_to, len(addresses), log_err)
	}
	return raw_logs, log_err
}

func (c *Client) filter_logs(query ethereum.FilterQuery, request_timeout time.Duration) ([]eth_types.Log, error) {
	ctx, cancel := context.WithTimeout(c.ctx, request_timeout)
	defer cancel()

	raw_logs, log_err := c.client.FilterLogs(ctx, query)
	if log_err != nil {
		c.decrease_rating()
		return nil, fmt.Errorf("FilterLogs query (%v): %w", query, log_err)
	}
	c.increase_rating()

	return raw_logs, nil
}
