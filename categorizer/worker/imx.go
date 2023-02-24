// imx smartcontract cateogirzer
// for documentation see:
// https://github.com/immutable/imx-core-sdk-golang/blob/6541766b54733580889f5051653d82f077c2aa17/imx/api/docs/TransfersApi.md#ListTransfers
// https://github.com/immutable/imx-core-sdk-golang/blob/6541766b54733580889f5051653d82f077c2aa17/imx/api/docs/MintsApi.md#listmints
package worker

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/blocklords/gosds/categorizer/imx"
	"github.com/blocklords/gosds/categorizer/smartcontract"
	"github.com/blocklords/gosds/categorizer/transaction"
	"github.com/blocklords/gosds/common/data_type/key_value"
	"github.com/blocklords/gosds/db"

	"github.com/blocklords/gosds/app/remote/message"

	imx_api "github.com/immutable/imx-core-sdk-golang/imx/api"
)

// we fetch transfers and mints.
// each will slow down the sleep time to the IMX open client API.
const IMX_REQUEST_TYPE_AMOUNT = 2

// Run the goroutine for each Imx smartcontract.
func ImxRun(db *db.Database, block *smartcontract.Smartcontract, manager *imx.Manager, broadcast chan message.Broadcast) {
	thisWorker := Worker{
		smartcontract:  block,
		broadcast_chan: broadcast,
		db:             db,
	}

	fmt.Println("imx worker " + block.Address + "." + block.NetworkId + " starting categorization")

	configuration := imx_api.NewConfiguration()
	apiClient := imx_api.NewAPIClient(configuration)

	for {
		timestamp := time.Unix(int64(block.CategorizedBlockTimestamp), 0).Format(time.RFC3339)

		broadcast_logs, err := categorize_imx_transfers(&thisWorker, apiClient, manager.DelayPerSecond, timestamp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error when calling `Imx.TransfersApi.ListTransfers``: %v\n", err)
			fmt.Println("trying to request again in 10 seconds...")
			time.Sleep(10 * time.Second)
			continue
		}

		// it should be mints
		mints, err := categorize_imx_mints(&thisWorker, apiClient, manager.DelayPerSecond, timestamp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error when calling `Imx.TransfersApi.ListTransfers``: %v\n", err)
			fmt.Println("trying to request again in 10 seconds...")
			time.Sleep(10 * time.Second)
			continue
		}

		broadcast_logs = append(broadcast_logs, mints...)

		broadcast_block_categorization(&thisWorker, broadcast_logs)
	}
}

// Returns list of transfers
func categorize_imx_transfers(worker *Worker, apiClient *imx_api.APIClient, sleep time.Duration, timestamp string) ([]map[string]interface{}, error) {
	status := "success"
	pageSize := imx.PAGE_SIZE
	orderBy := "transaction_id"
	direction := "asc"

	cursor := ""
	var resp *imx_api.ListTransfersResponse
	var r *http.Response
	var err error
	broadcastTransactions := make([]map[string]interface{}, 0)

	for {
		request := apiClient.TransfersApi.ListTransfers(context.Background()).MinTimestamp(timestamp).PageSize(pageSize)
		if cursor != "" {
			request = request.Cursor(cursor)
		}
		resp, r, err = request.OrderBy(orderBy).Direction(direction).Status(status).TokenAddress(worker.smartcontract.Address).Execute()
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

			tx := &transaction.Transaction{
				NetworkId:      "imx",
				Address:        worker.smartcontract.Address,
				BlockNumber:    uint64(blockTime.Unix()),
				BlockTimestamp: uint64(blockTime.Unix()),
				Txid:           strconv.Itoa(int(imxTx.TransactionId)),
				TxIndex:        uint(0),
				TxFrom:         imxTx.User,
				Method:         "Transfer",
				Args:           arguments,
				Value:          0.0,
			}

			for {
				createdErr := transaction.Save(worker.db, tx)
				if createdErr == nil {
					break
				}
				fmt.Println("error. failed to create an imx transaction row in the database")
				fmt.Println("this means either the database is down, or emergency bug due to code change")
				fmt.Println("if it's a code change, then stop the SDS Categorizer and fix the error asap")
				fmt.Println("otherwise, make the database alive asap.")
				fmt.Println("error message: ", createdErr)
				fmt.Println("waiting for 10 seconds, before attempting to save again...")

				time.Sleep(10 * time.Second)
			}

			// if mints or transfers update the same block number
			// we skip it.
			if int(blockTime.Unix()) > int(worker.smartcontract.CategorizedBlockNumber) {
				smartcontract.SetSyncing(worker.db, worker.smartcontract, uint64(blockTime.Unix()), uint64(blockTime.Unix()))
			}

			tx_kv, err := key_value.NewFromInterface(tx)
			if err != nil {
				return nil, fmt.Errorf("failed to serialize transaction to key-value %v: %v", tx, err)
			}

			broadcastTransactions = append(broadcastTransactions, tx_kv)
		}

		time.Sleep(sleep * IMX_REQUEST_TYPE_AMOUNT)

		if resp.Remaining == 0 {
			break
		} else if cursor != "" {
			cursor = resp.Cursor
		}
	}

	return broadcastTransactions, nil
}

func categorize_imx_mints(worker *Worker, apiClient *imx_api.APIClient, sleep time.Duration, timestamp string) ([]map[string]interface{}, error) {
	status := "success"
	pageSize := imx.PAGE_SIZE
	orderBy := "transaction_id"
	direction := "asc"

	cursor := ""
	var resp *imx_api.ListTransfersResponse
	var r *http.Response
	var err error
	broadcastTransactions := make([]map[string]interface{}, 0)

	for {
		request := apiClient.TransfersApi.ListTransfers(context.Background()).MinTimestamp(timestamp).PageSize(pageSize)
		if cursor != "" {
			request = request.Cursor(cursor)
		}
		resp, r, err = request.OrderBy(orderBy).Direction(direction).Status(status).TokenAddress(worker.smartcontract.Address).Execute()
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

			tx, err := transaction.ParseTransaction(map[string]interface{}{
				"network_id":      "imx",
				"address":         worker.smartcontract.Address,
				"block_number":    uint64(blockTime.Unix()),
				"block_timestamp": uint64(blockTime.Unix()),
				"txid":            strconv.Itoa(int(imxTx.TransactionId)),
				"tx_index":        uint64(0),
				"tx_from":         imxTx.User,
				"method":          "Transfer",
				"arguments":       arguments,
				"value":           0.0,
			})
			if err != nil {
				fmt.Println("failed to parse the transaction. the imx response has ben changed")
				fmt.Println(err)
				continue
			}

			for {
				createdErr := transaction.Save(worker.db, tx)
				if createdErr == nil {
					break
				}
				fmt.Println("error. failed to create an imx transaction row in the database")
				fmt.Println("this means either the database is down, or emergency bug due to code change")
				fmt.Println("if it's a code change, then stop the SDS Categorizer and fix the error asap")
				fmt.Println("otherwise, make the database alive asap.")
				fmt.Println("error message: ", createdErr)
				fmt.Println("waiting for 10 seconds, before attempting to save again...")

				time.Sleep(10 * time.Second)
			}

			// if mints or transfers update the same block number
			// we skip it.
			if int(blockTime.Unix()) > int(worker.smartcontract.CategorizedBlockNumber) {
				smartcontract.SetSyncing(worker.db, worker.smartcontract, uint64(blockTime.Unix()), uint64(blockTime.Unix()))
			}

			tx_kv, err := key_value.NewFromInterface(tx)
			if err != nil {
				return nil, fmt.Errorf("failed to serialize transaction to key-value %v: %v", tx, err)
			}

			broadcastTransactions = append(broadcastTransactions, tx_kv)
		}
		time.Sleep(sleep * IMX_REQUEST_TYPE_AMOUNT)

		if resp.Remaining == 0 {
			break
		} else if cursor != "" {
			cursor = resp.Cursor
		}
	}

	return broadcastTransactions, nil
}
