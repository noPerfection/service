package handler

import (
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/categorizer/event"
	"github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/common/data_type/database"
	"github.com/blocklords/sds/common/data_type/key_value"
)

// on_categorize command handles an update of the smartcontracts
// as well as inserts new decoded event logs in the database.
//
// Send categorizer/handler.CATEGORIZER command from network categorizer sub service
// to execute this function.
func on_categorize(request message.Request, logger log.Logger, app_parameters ...interface{}) message.Reply {
	if app_parameters == nil || len(app_parameters) < 1 {
		return message.Fail("invalid parameters were given atleast database should be passed")
	}

	db_con, ok := app_parameters[0].(*remote.ClientSocket)
	if !ok {
		return message.Fail("missing Manager in the parameters")
	}

	raw_smartcontracts, _ := request.Parameters.GetKeyValueList("smartcontracts")
	smartcontracts := make([]*smartcontract.Smartcontract, len(raw_smartcontracts))

	for i, raw := range raw_smartcontracts {
		sm, err := smartcontract.New(raw)
		if err != nil {
			logger.Fatal("unexpected error. failed to deserialize smartcontract: ", err)
		}
		smartcontracts[i] = sm
	}

	raw_logs, _ := request.Parameters.GetKeyValueList("logs")

	logs := make([]*event.Log, len(raw_logs))
	for i, raw := range raw_logs {
		log, _ := event.NewFromMap(raw)
		logs[i] = log
	}

	for _, sm := range smartcontracts {
		var crud database.Crud = sm
		err := crud.Update(db_con, smartcontract.UPDATE_BLOCK_HEADER)
		if err != nil {
			return message.Fail("smartcontract.SaveBlockParameters: " + err.Error())
		}
	}

	for _, l := range logs {
		var crud database.Crud = l
		err := crud.Insert(db_con)
		if err != nil {
			return message.Fail("event.Insert: " + err.Error())
		}
	}

	return message.Reply{
		Status:     message.OK,
		Message:    "",
		Parameters: key_value.Empty(),
	}
}
