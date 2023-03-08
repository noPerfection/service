package categorizer

import (
	"fmt"
	debug_log "log"

	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/categorizer/event"
	"github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/db"

	zmq "github.com/pebbe/zmq4"
)

func NewCategorizerPusher() (*zmq.Socket, error) {
	sock, err := zmq.NewSocket(zmq.PUSH)
	if err != nil {
		return nil, err
	}

	url := "cat"
	if err := sock.Bind("inproc://" + url); err != nil {
		return nil, fmt.Errorf("trying to create categorizer connecting pusher: %v", err)
	}

	return sock, nil
}

// Sets up the socket that will be connected by the blockchain/categorizers
// The blockchain categorizers will set up the smartcontract informations on the database
func SetupSocket(database *db.Database) {
	sock, err := zmq.NewSocket(zmq.PULL)
	if err != nil {
		panic(err)
	}

	url := "cat"
	if err := sock.Connect("inproc://" + url); err != nil {
		debug_log.Fatalf("trying to create categorizer socket: %v", err)
	}

	for {
		// Wait for reply.
		msgs, _ := sock.RecvMessage(0)
		request, _ := message.ParseRequest(msgs)

		raw_smartcontracts, _ := request.Parameters.GetKeyValueList("smartcontracts")
		smartcontracts := make([]*smartcontract.Smartcontract, len(raw_smartcontracts))

		for i, raw := range raw_smartcontracts {
			sm, _ := smartcontract.New(raw)
			smartcontracts[i] = sm
		}

		raw_logs, _ := request.Parameters.GetKeyValueList("logs")

		logs := make([]*event.Log, len(raw_logs))
		for i, raw := range raw_logs {
			log, _ := event.NewFromMap(raw)
			logs[i] = log
		}

		for _, sm := range smartcontracts {
			err := smartcontract.SetSyncing(database, sm, sm.BlockNumber, sm.BlockTimestamp)
			if err != nil {
				panic(err)
			}
		}

		for _, l := range logs {
			err := event.Save(database, l)
			if err != nil {
				panic(err)
			}
		}
	}
}
