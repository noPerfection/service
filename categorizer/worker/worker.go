// EVM blockchain worker
package worker

import (
	"github.com/blocklords/gosds/categorizer/smartcontract"
	"github.com/blocklords/gosds/db"
	"github.com/blocklords/gosds/static/smartcontract/key"
)

type Worker struct {
	Db *db.Database

	Smartcontract *smartcontract.Smartcontract
}

// Print the log
func (worker *Worker) Prefix() string {
	k := key.New(worker.Smartcontract.NetworkId, worker.Smartcontract.Address)
	return "categorizer " + k.ToString() + ": "
}

func New(db *db.Database, sm *smartcontract.Smartcontract) *Worker {
	worker := Worker{
		Smartcontract: sm,
		Db:            db,
	}

	return &worker
}

// Create a new worker
func NewImxWorker(db *db.Database, sm *smartcontract.Smartcontract) *Worker {
	worker := Worker{
		Smartcontract: sm,
		Db:            db,
	}

	return &worker
}
