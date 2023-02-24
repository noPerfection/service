// Spaghetti Worker connects to the blockchain over the loop.
// Worker is running per blockchain network with VM.
package worker

import (
	"fmt"
	"time"

	"github.com/blocklords/gosds/spaghetti/block"
	"github.com/blocklords/gosds/spaghetti/network_client"

	"github.com/blocklords/gosds/app/remote/message"

	"github.com/blocklords/gosds/common/data_type"
	"github.com/blocklords/gosds/common/data_type/key_value"
)

// the global variables that we pass between functions in this worker.
// the functions are recursive.
type SpaghettiWorker struct {
	block_number      uint64
	client            *network_client.NetworkClient
	broadcast_channel chan message.Broadcast
	debug             bool
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

// A new SpaghettiWorker
func New(client *network_client.NetworkClient, block_number uint64, broadcast_channel chan message.Broadcast, debug bool) *SpaghettiWorker {
	return &SpaghettiWorker{
		client:            client,
		block_number:      block_number,
		broadcast_channel: broadcast_channel,
		debug:             debug,
	}
}

// run the worker as a goroutine.
// the channel is used to receive the data necessary for running goroutine.
//
// the channel should pass three arguments:
// - block number
// - network id
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

		set_err := broadcast_block(worker, block)
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
func broadcast_block(worker *SpaghettiWorker, b *block.Block) error {
	worker.block_number = b.BlockNumber

	worker.log_debug(fmt.Sprintf("broadcast the new block %d", b.BlockNumber))

	new_reply := message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"network_id":      worker.client.Network.Id,
			"block_number":    b.BlockNumber,
			"block_timestamp": b.BlockTimestamp,
			"logs":            data_type.ToMapList(b.Logs),
		}),
	}

	worker.log_debug(fmt.Sprintf("broadcasting network id %s, block number %d", worker.client.Network.Id, b.BlockNumber))

	worker.broadcast_channel <- message.NewBroadcast(worker.client.Network.Id+" ", new_reply)

	return nil
}
