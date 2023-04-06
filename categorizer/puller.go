package categorizer

import (
	"fmt"

	"github.com/blocklords/sds/app/command"
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/blockchain/handler"
	"github.com/blocklords/sds/categorizer/event"
	"github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/db"

	zmq "github.com/pebbe/zmq4"
)

func CategorizerPullEndpoint() string {
	return "inproc://cat"
}

// To connect to the categorizer
// to update data on the database.
func NewCategorizerPusher() (*zmq.Socket, error) {
	sock, err := zmq.NewSocket(zmq.PUSH)
	if err != nil {
		return nil, err
	}

	if err := sock.Connect(CategorizerPullEndpoint()); err != nil {
		return nil, fmt.Errorf("trying to create categorizer connecting pusher: %v", err)
	}

	return sock, nil
}

// Opens up the socket to receive decoded event logs.
// The received data stored in the database.
// This socket receives messages from blockchain/categorizers.
func RunPuller(cat_logger log.Logger, database *db.Database) {
	service, err := service.InprocessFromUrl(CategorizerPullEndpoint())
	if err != nil {
		cat_logger.Fatal("failed to create inproc service from url", "error", err)
	}
	reply, err := controller.NewPull(service, cat_logger)
	if err != nil {
		cat_logger.Fatal("failed to create pull controller", "error", err)
	}

	handlers := command.EmptyHandlers().
		Add(handler.NEW_CATEGORIZED_SMARTCONTRACTS, on_new_smartcontracts)
	err = reply.Run(handlers, database)
	if err != nil {
		cat_logger.Fatal("failed to run reply controller", "error", err)
	}
}

func on_new_smartcontracts(request message.Request, logger log.Logger, parameters ...interface{}) message.Reply {
	if parameters == nil || len(parameters) < 1 {
		return message.Fail("invalid parameters were given atleast manager should be passed")
	}

	database, ok := parameters[0].(*db.Database)
	if !ok {
		return message.Fail("missing Manager in the parameters")
	}

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

	return message.Reply{
		Status:     message.OK,
		Message:    "",
		Parameters: key_value.Empty(),
	}
}
