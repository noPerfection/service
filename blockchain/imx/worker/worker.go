package worker

import (
	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/blockchain/imx/client"
)

// the global variables that we pass between functions in this worker.
// the functions are recursive.
type SpaghettiWorker struct {
	client            *client.Client
	broadcast_channel chan message.Broadcast
	debug             bool
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
}
