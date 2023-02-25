// EVM blockchain worker
package worker

import (
	"github.com/blocklords/gosds/categorizer/abi"
	"github.com/blocklords/gosds/categorizer/smartcontract"
	"github.com/blocklords/gosds/db"
	"github.com/blocklords/gosds/static/smartcontract/key"

	"github.com/blocklords/gosds/app/remote/message"
)

type Worker struct {
	Db *db.Database

	Smartcontract  *smartcontract.Smartcontract
	broadcast_chan chan message.Broadcast
}

// Print the log
func (worker *Worker) Prefix() string {
	k := key.New(worker.Smartcontract.NetworkId, worker.Smartcontract.Address)
	return "categorizer " + k.ToString() + ": "
}

func New(db *db.Database, sm *smartcontract.Smartcontract, broadcast chan message.Broadcast) *Worker {
	worker := Worker{
		Smartcontract:  sm,
		broadcast_chan: broadcast,
		Db:             db,
	}

	return &worker
}

// Create a new worker
func NewImxWorker(db *db.Database, sm *smartcontract.Smartcontract, broadcast chan message.Broadcast) *Worker {
	worker := Worker{
		Smartcontract:  sm,
		broadcast_chan: broadcast,
		Db:             db,
	}

	return &worker
}

// Create a new worker
func NewEvmWorker(db *db.Database, abi *abi.Abi, worker_smartcontract *smartcontract.Smartcontract, broadcast chan message.Broadcast, in chan RequestSpaghettiBlockRange, out chan ReplySpaghettiBlockRange, log_parse_in chan RequestLogParse, log_parse_out chan ReplyLogParse) *Worker {
	worker := Worker{
		Smartcontract:             worker_smartcontract,
		broadcast_chan:            broadcast,
		Db:                        db,
		spaghetti_block_range_in:  in,
		spaghetti_block_range_out: out,
		log_parse_in:              log_parse_in,
		log_parse_out:             log_parse_out,
		spaghetti_sub_socket:      nil,
		abi:                       abi,
	}

	return &worker
}
