package client

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/blocklords/gosds/blockchain/event"
	"github.com/blocklords/gosds/blockchain/imx"
	"github.com/blocklords/gosds/blockchain/imx/util"
	"github.com/blocklords/gosds/blockchain/network"
	"github.com/blocklords/gosds/common/data_type/key_value"

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
func (client *Client) GetSmartcontractTransferLogs(address string, sleep time.Duration, timestamp string) ([]*event.Log, error) {
	status := "success"
	pageSize := imx.PAGE_SIZE
	orderBy := "transaction_id"
	direction := "asc"

	cursor := ""
	var resp *imx_api.ListTransfersResponse
	var r *http.Response
	var err error
	logs := make([]*event.Log, 0)

	for {
		request := client.client.TransfersApi.ListTransfers(client.ctx).MinTimestamp(timestamp).PageSize(pageSize)

		if cursor != "" {
			request = request.Cursor(cursor)
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
			timestamp = imxTx.GetTimestamp()
			blockTime, err := time.Parse(time.RFC3339, timestamp)
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

			// todo change the imx to store in the log
			l := &event.Log{
				NetworkId:      "imx",
				Txid:           strconv.Itoa(int(imxTx.TransactionId)),
				BlockNumber:    uint64(blockTime.Unix()),
				BlockTimestamp: uint64(blockTime.Unix()),
				LogIndex:       uint(i),
				Data:           data_string,
				Topics:         []string{},
				Address:        address,
			}

			logs = append(logs, l)
		}

		time.Sleep(sleep)

		if resp.Remaining == 0 {
			break
		} else if cursor != "" {
			cursor = resp.Cursor
		}
	}
	fmt.Println("return the parameters")

	return logs, nil
}

func (client *Client) GetSmartcontractMintLogs(address string, sleep time.Duration, timestamp string) ([]*event.Log, error) {
	status := "success"
	pageSize := imx.PAGE_SIZE
	orderBy := "transaction_id"
	direction := "asc"

	cursor := ""
	var resp *imx_api.ListMintsResponse
	var r *http.Response
	var err error
	logs := make([]*event.Log, 0)

	for {
		request := client.client.MintsApi.ListMints(context.Background()).MinTimestamp(timestamp).PageSize(pageSize)
		if cursor != "" {
			request = request.Cursor(cursor)
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
			timestamp = imxTx.GetTimestamp()
			blockTime, err := time.Parse(time.RFC3339, timestamp)
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
			fmt.Printf("data: %s", data_string)
			// todo change the imx to store in the log
			l := &event.Log{
				NetworkId:      "imx",
				Txid:           strconv.Itoa(int(imxTx.TransactionId)),
				BlockNumber:    uint64(blockTime.Unix()),
				BlockTimestamp: uint64(blockTime.Unix()),
				LogIndex:       uint(i),
				Data:           data_string,
				Topics:         []string{},
				Address:        address,
			}

			logs = append(logs, l)
		}
		// time.Sleep(sleep * req_per_second)

		fmt.Println("cursor", resp.Cursor)

		if resp.Remaining == 0 {
			break
		} else if cursor != "" {
			cursor = resp.Cursor
		}
	}

	return logs, nil
}
