// Spaghetti Worker connects to the blockchain over the loop.
// Worker is running per blockchain network with VM.
package worker

import (
	"fmt"
	"time"

	"github.com/blocklords/gosds/blockchain/evm/block"
	"github.com/blocklords/gosds/blockchain/evm/client"

	"github.com/blocklords/gosds/app/remote/message"

	"github.com/blocklords/gosds/common/data_type"
)

// the global variables that we pass between functions in this worker.
// the functions are recursive.
type SpaghettiWorker struct {
	client            *client.Client
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
func New(client *client.Client, broadcast_channel chan message.Broadcast, debug bool) *SpaghettiWorker {
	return &SpaghettiWorker{
		client:            client,
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

	block_number, err := worker.client.GetRecentBlockNumber()
	if err != nil {
		panic(err)
	}

	// optimize in case of the error
	// or slow internet connection
	// we need to get the data as fast as possible
	for {
		block, err := worker.client.GetBlock(block_number)
		if err != nil {
			println(worker.log_prefix(), `failed to get the block `, block_number, " from provider for network id ", worker.client.Network.Id, ". received error: ", err.Error())
			println(worker.log_prefix(), `waiting for 10 seconds before trying again...`)
			time.Sleep(10 * time.Second)
			continue
		}

		worker.broadcast_block(block)

		time.Sleep(1 * time.Second)

		block_number++
	}
}

// this function saves the b
func (worker *SpaghettiWorker) broadcast_block(b *block.Block) {
	new_reply := message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: map[string]interface{}{
			"network_id":      worker.client.Network.Id,
			"block_number":    b.BlockNumber,
			"block_timestamp": b.BlockTimestamp,
			"logs":            data_type.ToMapList(b.Logs),
		},
	}

	worker.log_debug(fmt.Sprintf("broadcasting network id %s, block number %d", worker.client.Network.Id, b.BlockNumber))

	worker.broadcast_channel <- message.NewBroadcast(worker.client.Network.Id+" ", new_reply)
}
