package client

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/blocklords/sds/blockchain/event"
	"github.com/blocklords/sds/blockchain/imx"
	"github.com/blocklords/sds/blockchain/imx/util"
	"github.com/blocklords/sds/blockchain/network"
	"github.com/blocklords/sds/blockchain/transaction"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/static/smartcontract/key"

	imx_api "github.com/immutable/imx-core-sdk-golang/imx/api"
)

type Client struct {
	client  *imx_api.APIClient
	ctx     context.Context
	Network *network.Network
}

// Create a network client connected to the blockchain based on a Static parameters
// Static parameters include the node url
func New(network *network.Network) *Client {
	configuration := imx_api.NewConfiguration()
	client := imx_api.NewAPIClient(configuration)
	ctx := context.TODO()

	return &Client{
		client:  client,
		ctx:     ctx,
		Network: network,
	}
}

// Returns list of transfers
func (client *Client) GetSmartcontractTransferLogs(address string, sleep time.Duration, timestamp string, timestamp_to string) ([]*event.RawLog, error) {
	status := "success"
	pageSize := imx.PAGE_SIZE
	orderBy := "transaction_id"
	direction := "asc"

	var resp *imx_api.ListTransfersResponse
	var r *http.Response
	var err error
	logs := make([]*event.RawLog, 0)

	till_max := false
	if strings.Compare(timestamp, timestamp_to) != 0 {
		till_max = true
	}

	for {
		request := client.client.TransfersApi.ListTransfers(client.ctx).MinTimestamp(timestamp).PageSize(pageSize)
		if till_max {
			request = request.MaxTimestamp(timestamp_to)
		}

		resp, r, err = request.OrderBy(orderBy).Direction(direction).Status(status).TokenAddress(address).Execute()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error when calling `Imx.TransfersApi.ListTransfers``: %v\n", err)
			fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
			fmt.Println("trying to request again in 10 seconds...")
			time.Sleep(10 * time.Second)
			return nil, fmt.Errorf("failed to fetch transfers list: %v", err)
		}

		for i, imxTx := range resp.Result {
			blockTime, err := time.ParseInLocation(time.RFC3339, imxTx.GetTimestamp(), time.UTC)
			next_time := blockTime.Add(time.Second)
			timestamp = next_time.UTC().Format(time.RFC3339)

			if err != nil {
				return nil, fmt.Errorf("error, parsing transaction data error: %v", err)
			}

			// eth transfers are not supported yet
			if imxTx.Token.Type == "ETH" {
				fmt.Println("skip, the SDS doesn't support transfer of ETH native tokens")
				continue
			}

			var arguments = make(map[string]interface{}, 3)
			arguments["from"] = imxTx.User
			arguments["to"] = imxTx.Receiver

			if imxTx.Token.Type == "ERC721" {
				arguments["tokenId"] = imxTx.Token.Data.TokenId
			} else {
				value, err := util.Erc20Amount(&imxTx.Token.Data)
				if err != nil {
					return nil, fmt.Errorf("failed to decode erc20 value: %w", err)
				}

				arguments["value"] = value
			}

			data := key_value.Empty().Set("log", "Transfer").Set("outputs", arguments)
			data_string, err := data.ToString()
			if err != nil {
				return nil, fmt.Errorf("failed to serialize the key-value to string: %w", err)
			}

			key := key.New(imx.NETWORK_ID, address)
			block := blockchain.New(uint64(blockTime.UTC().Unix()), uint64(blockTime.UTC().Unix()))
			tx_key := blockchain.TransactionKey{
				Id:    strconv.Itoa(int(imxTx.TransactionId)),
				Index: uint(i),
			}

			transaction := transaction.RawTransaction{
				Key:            key,
				Block:          block,
				TransactionKey: tx_key,
				Value:          0,
				Data:           "",
			}

			l := &event.RawLog{
				Transaction: transaction,
				LogIndex:    uint(i),
				Data:        data_string,
				Topics:      []string{},
			}

			logs = append(logs, l)
		}

		if !till_max {
			break
		}

		if resp.Remaining == 0 {
			break
		}

		time.Sleep(sleep)
	}

	return logs, nil
}

func (client *Client) GetSmartcontractMintLogs(address string, sleep time.Duration, timestamp string, timestamp_to string) ([]*event.RawLog, error) {
	status := "success"
	pageSize := imx.PAGE_SIZE
	orderBy := "transaction_id"
	direction := "asc"

	var resp *imx_api.ListMintsResponse
	var r *http.Response
	var err error
	logs := make([]*event.RawLog, 0)

	till_max := false
	if strings.Compare(timestamp, timestamp_to) != 0 {
		till_max = true
	}

	for {
		request := client.client.MintsApi.ListMints(context.Background()).MinTimestamp(timestamp).PageSize(pageSize)
		if till_max {
			request = request.MaxTimestamp(timestamp_to)
		}

		resp, r, err = request.OrderBy(orderBy).Direction(direction).Status(status).TokenAddress(address).Execute()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error when calling `Imx.TransfersApi.ListTransfers``: %v\n", err)
			fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
			fmt.Println("trying to request again in 10 seconds...")
			time.Sleep(10 * time.Second)
			return nil, fmt.Errorf("failed to fetch transfers list: %v", err)
		}

		for i, imxTx := range resp.Result {
			blockTime, err := time.ParseInLocation(time.RFC3339, imxTx.GetTimestamp(), time.UTC)
			next_time := blockTime.Add(time.Second)
			timestamp = next_time.UTC().Format(time.RFC3339)

			if err != nil {
				return nil, fmt.Errorf("error, parsing transaction data error: %v", err)
			}

			// eth transfers are not supported yet
			if imxTx.Token.Type == "ETH" {
				fmt.Println("skip, the SDS doesn't support minting of ETH native tokens")
				continue
			}

			arguments := map[string]interface{}{
				"from": "0x0000000000000000000000000000000000000000",
				"to":   imxTx.User,
			}

			if imxTx.Token.Type == "ERC721" {
				arguments["tokenId"] = imxTx.Token.Data.TokenId
			} else {
				value, err := util.Erc20Amount(&imxTx.Token.Data)
				if err != nil {
					return nil, err
				}

				arguments["value"] = value
			}

			data := key_value.Empty().Set("log", "Transfer").Set("outputs", arguments)
			data_string, err := data.ToString()
			if err != nil {
				return nil, fmt.Errorf("failed to serialize the key-value to string: %w", err)
			}

			key := key.New(imx.NETWORK_ID, address)
			block := blockchain.New(uint64(blockTime.UTC().Unix()), uint64(blockTime.UTC().Unix()))
			tx_key := blockchain.TransactionKey{
				Id:    strconv.Itoa(int(imxTx.TransactionId)),
				Index: uint(i),
			}

			transaction := transaction.RawTransaction{
				Key:            key,
				Block:          block,
				TransactionKey: tx_key,
				Value:          0,
				Data:           "",
			}

			l := &event.RawLog{
				Transaction: transaction,
				LogIndex:    uint(i),
				Data:        data_string,
				Topics:      []string{},
			}

			logs = append(logs, l)
		}

		if !till_max {
			break
		}

		if resp.Remaining == 0 {
			break
		}

		time.Sleep(sleep)
	}

	return logs, nil
}
