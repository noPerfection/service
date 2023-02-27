package client

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/blocklords/gosds/blockchain/imx"
	"github.com/blocklords/gosds/blockchain/network"
	"github.com/blocklords/gosds/categorizer/log"

	imx_api "github.com/immutable/imx-core-sdk-golang/imx/api"
)

type Client struct {
	client  *imx_api.APIClient
	ctx     context.Context
	Network *network.Network
}

// Create a network client connected to the blockchain based on a Static parameters
// Static parameters include the node url
func New(network *network.Network) (*Client, error) {
	configuration := imx_api.NewConfiguration()
	client := imx_api.NewAPIClient(configuration)
	ctx := context.TODO()

	return &Client{
		client:  client,
		ctx:     ctx,
		Network: network,
	}, nil
}

// Returns list of transfers
func (client *Client) GetSmartcontractTransferLogs(req_per_second time.Duration, address string, sleep time.Duration, timestamp string) ([]*log.Log, error) {
	status := "success"
	pageSize := imx.PAGE_SIZE
	orderBy := "transaction_id"
	direction := "asc"

	cursor := ""
	var resp *imx_api.ListTransfersResponse
	var r *http.Response
	var err error
	logs := make([]*log.Log, 0)

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

		for _, imxTx := range resp.Result {
			blockTime, err := time.Parse(time.RFC3339, imxTx.GetTimestamp())
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
				value, err := imx.Erc20Amount(&imxTx.Token.Data)
				if err != nil {
					return nil, err
				}

				arguments["value"] = value
			}

			// todo change the imx to store in the log
			l := &log.Log{
				NetworkId:      "imx",
				Address:        address,
				BlockNumber:    uint64(blockTime.Unix()),
				BlockTimestamp: uint64(blockTime.Unix()),
				Txid:           strconv.Itoa(int(imxTx.TransactionId)),
				LogIndex:       uint(0),
				Log:            "Transfer",
				Output:         arguments,
				// TxFrom:         imxTx.User,
				// Value:          0.0,
			}

			logs = append(logs, l)
		}

		time.Sleep(sleep * req_per_second)

		if resp.Remaining == 0 {
			break
		} else if cursor != "" {
			cursor = resp.Cursor
		}
	}

	return logs, nil
}

func (client *Client) GetSmartcontractMintLogs(req_per_second time.Duration, address string, sleep time.Duration, timestamp string) ([]*log.Log, error) {
	status := "success"
	pageSize := imx.PAGE_SIZE
	orderBy := "transaction_id"
	direction := "asc"

	cursor := ""
	var resp *imx_api.ListTransfersResponse
	var r *http.Response
	var err error
	logs := make([]*log.Log, 0)

	for {
		request := client.client.TransfersApi.ListTransfers(context.Background()).MinTimestamp(timestamp).PageSize(pageSize)
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

		for _, imxTx := range resp.Result {
			blockTime, err := time.Parse(time.RFC3339, imxTx.GetTimestamp())
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
				value, err := imx.Erc20Amount(&imxTx.Token.Data)
				if err != nil {
					return nil, err
				}

				arguments["value"] = value
			}

			l := &log.Log{
				NetworkId:      "imx",
				Address:        address,
				BlockNumber:    uint64(blockTime.Unix()),
				BlockTimestamp: uint64(blockTime.Unix()),
				Txid:           strconv.Itoa(int(imxTx.TransactionId)),
				LogIndex:       uint(0),
				Log:            "Transfer",
				Output:         arguments,
			}

			logs = append(logs, l)
		}
		time.Sleep(sleep * req_per_second)

		if resp.Remaining == 0 {
			break
		} else if cursor != "" {
			cursor = resp.Cursor
		}
	}

	return logs, nil
}
