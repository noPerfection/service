package categorizer

import (
	"fmt"

	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/categorizer/event"
	"github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/db"

	zmq "github.com/pebbe/zmq4"
)

// To connect to the categorizer
// to update data on the database.
func NewCategorizerPusher() (*zmq.Socket, error) {
	sock, err := zmq.NewSocket(zmq.PUSH)
	if err != nil {
		return nil, err
	}

	if err := sock.Connect("inproc://cat"); err != nil {
		return nil, fmt.Errorf("trying to create categorizer connecting pusher: %v", err)
	}

	return sock, nil
}

// Opens up the socket to receive decoded event logs.
// The received data stored in the database.
// This socket receives messages from blockchain/categorizers.
func RunPuller(cat_logger log.Logger, database *db.Database) {
	logger, err := cat_logger.ChildWithoutReport("puller")
	if err != nil {
		cat_logger.Fatal("failed to create child logger", "error", err)
	}

	sock, err := zmq.NewSocket(zmq.PULL)
	if err != nil {
		logger.Fatal("zmq.NewSocket", "error", err)
	}

	url := "inproc://cat"
	if err := sock.Bind(url); err != nil {
		logger.Fatal("trying to create categorizer socket: %v", "url", url, "error", err)
	}

	logger.Info("Puller waits for the messages", "url", url)

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
			err := smartcontract.SaveBlockParameters(database, sm)
			if err != nil {
				logger.Fatal("smartcontract.SaveBlockParameters", "error", err)
			}
		}

		for _, l := range logs {
			err := event.Save(database, l)
			if err != nil {
				logger.Fatal("event.Save", "error", err)
			}
		}
	}
}
