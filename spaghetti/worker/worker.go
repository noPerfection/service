// Spaghetti Worker connects to the blockchain over the loop.
// Worker is running per blockchain network with VM.
package worker

import (
	"errors"
	"fmt"
	"time"

	"github.com/blocklords/gosds/db"
	"github.com/blocklords/gosds/spaghetti/block"
	"github.com/blocklords/gosds/spaghetti/log"
	"github.com/blocklords/gosds/spaghetti/network_client"
	"github.com/blocklords/gosds/spaghetti/transaction"

	"github.com/blocklords/gosds/app/env"

	"github.com/blocklords/gosds/app/remote/message"

	"github.com/blocklords/gosds/common/data_type"
	"github.com/blocklords/gosds/common/data_type/key_value"
)

// the global variables that we pass between functions in this worker.
// the functions are recursive.
type SpaghettiWorker struct {
	block_number        uint64
	database_connection *db.Database
	client              *network_client.NetworkClient
	broadcast_channel   chan message.Broadcast
	debug               bool
}

// Differentiate the workers from each other
// We have multiple workers running concurrently.
func (worker *SpaghettiWorker) log_prefix() string {
	return "worker network_id: " + worker.client.Network.Id + ": "
}

// Print the logs on stdout or not
func (worker *SpaghettiWorker) log_debug(message string) {
	if worker.debug {
		println(worker.log_prefix(), message)
	}
}

// This function updates the block number for a network id.
//
// Then it sends the updated block number to the broadcaster.
func broadcast_new_block(worker *SpaghettiWorker, b *block.Block) error {
	new_reply := message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"network_id":      worker.client.Network.Id,
			"block_number":    b.BlockNumber,
			"block_timestamp": b.BlockTimestamp,
			"transactions":    data_type.ToMapList(b.Transactions),
			"logs":            data_type.ToMapList(b.Logs),
		}),
	}

	worker.log_debug(fmt.Sprintf("broadcasting network id %s, block number %d", worker.client.Network.Id, b.BlockNumber))

	worker.broadcast_channel <- message.NewBroadcast(worker.client.Network.Id+" ", new_reply)

	return nil
}

// A new SpaghettiWorker
func New(client *network_client.NetworkClient, block_number uint64, db *db.Database, broadcast_channel chan message.Broadcast, debug bool) *SpaghettiWorker {
	return &SpaghettiWorker{
		client:              client,
		block_number:        block_number,
		database_connection: db,
		broadcast_channel:   broadcast_channel,
		debug:               debug,
	}
}

// run the worker as a goroutine.
// the channel is used to receive the data necessary for running goroutine.
//
// the channel should pass three arguments:
// - block number
// - network id
// - db connection
func (worker *SpaghettiWorker) Sync() {
	worker.log_debug("worker for network id " + worker.client.Network.Id + " started!\n\n")
	recentBlockNumber, err := worker.client.GetRecentBlockNumber()
	worker.log_debug("provider responded ")
	if err != nil {
		println(worker.log_prefix(), `Failed to get block from provider for network id `, worker.client.Network.Id, ", received error: ", err.Error())
		println(worker.log_prefix(), `Waiting for a 10 seconds and tring again...`)
		time.Sleep(10 * time.Second)
		worker.Sync()
		return
	}

	left := recentBlockNumber - worker.block_number
	if left > 0 {
		worker.log_debug(fmt.Sprintf("sync blocks %d", left))
		sync_till(worker, worker.block_number+1, recentBlockNumber)
	} else {
		// since we synced all blocks, let's wait for 10 seconds
		// and check for a new mined block
		time.Sleep(10 * time.Second)
	}

	worker.log_debug("re-sync")
	worker.Sync()
}

// this function syncs SDS Spaghetti with blockchain.
func sync_till(worker *SpaghettiWorker, blockFrom uint64, block_number_to uint64) {
	for block_number := blockFrom; block_number <= block_number_to; block_number++ {
		block, err := worker.client.GetBlock(block_number)
		if err != nil {
			println(worker.log_prefix(), `failed to get the block `, block_number, " from provider for network id ", worker.client.Network.Id, ". received error: ", err.Error())
			println(worker.log_prefix(), `waiting for 10 seconds before trying again...`)
			time.Sleep(10 * time.Second)
			sync_till(worker, block_number, block_number_to)
			return
		}

		set_err := sync_block(worker, block)
		if set_err != nil {
			println(worker.log_prefix(), `failed to save the block information for block `, block_number, " in provider for network id ", worker.client.Network.Id, ". received error: ", set_err.Error())
			println(worker.log_prefix(), `waiting for 10 seconds before trying again...`)
			time.Sleep(10 * time.Second)
			sync_till(worker, block_number, block_number_to)
			return
		}

		time.Sleep(time.Second * 1)
	}
}

// this function saves the b
func sync_block(worker *SpaghettiWorker, b *block.Block) error {
	worker.log_debug("sync the database with a spaghetti data")

	if err := clear(worker, b); err != nil {
		return fmt.Errorf("before syncing the block, cleaning: %v", err)
	}

	var transaction_amount uint = uint(len(b.Transactions))
	var log_amount uint = uint(len(b.Logs))

	// save the block number in the database
	err := block.SetBlock(worker.database_connection, worker.client.Network.Id, b.BlockNumber, transaction_amount, log_amount, b.BlockTimestamp)
	if err != nil {
		return err
	}
	worker.log_debug(fmt.Sprintf("set the block %d, tx amount: %d, has error %b", b.BlockNumber, transaction_amount, err))

	worker.block_number = b.BlockNumber

	transaction_err := SaveTransactions(worker.database_connection, b.Transactions)
	if transaction_err != nil {
		return transaction_err
	}

	log_err := SaveLogs(worker.database_connection, b.Logs)
	if log_err != nil {
		return log_err
	}

	worker.log_debug(fmt.Sprintf("broadcast the new block %d", b.BlockNumber))

	return broadcast_new_block(worker, b)
}

// saves the transactions in the database for a given block
func SaveTransactions(db *db.Database, transactions []*transaction.Transaction) error {
	for _, tx := range transactions {
		err := transaction.DbSave(db, tx)
		if err != nil {
			return errors.New(`failed to save in the database the transaction for network id ` + tx.NetworkId + `, tx id ` + tx.Txid + `, received error: ` + err.Error())
		}
	}
	return nil
}

// saves the logs in the database for a given block
func SaveLogs(db *db.Database, logs []*log.Log) error {
	for _, l := range logs {
		err := log.DbSave(db, l)
		if err != nil {
			return err
		}
	}

	return nil
}

// We clear the old data from the Database
// due to the structure of our database, we clear the data
// in the following order:
// - log
// - transaction
// - block
func clear(worker *SpaghettiWorker, b *block.Block) error {
	cache_duration := uint64(env.GetNumeric("SDS_SPAGHETTI_CACHE_DURATION"))
	if b.BlockTimestamp < cache_duration {
		return errors.New(worker.log_prefix() + ": the timestamp for a block is invalid")
	}

	save_time := b.BlockTimestamp - cache_duration

	latest_block_number, err := block.GetLatestBlockNumber(worker.database_connection, worker.client.Network.Id, save_time)
	if err != nil {
		return fmt.Errorf("clearing failed; failed to fetch the latest block number before the block timestamp: %v", err)
	}
	// unlikely, but we are paranoid, so still let's check it
	if latest_block_number == 0 {
		worker.log_debug("clearing is not needed, there is no older blocks than a timestamp")
		return nil
	}

	// clear the old logs
	if err := log.DbClear(worker.database_connection, worker.client.Network.Id, latest_block_number); err != nil {
		return err
	}

	// clear the old transactions
	if err := transaction.DbClear(worker.database_connection, worker.client.Network.Id, latest_block_number); err != nil {
		return err
	}

	// clear the old blocks
	if err := block.Clear(worker.database_connection, worker.client.Network.Id, latest_block_number); err != nil {
		return err
	}

	return nil
}
