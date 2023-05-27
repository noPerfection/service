package handler

import (
	"github.com/blocklords/sds/app/communication/message"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/common/data_type/database"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/indexer/event"
	"github.com/blocklords/sds/indexer/smartcontract"
)

// on_categorize command handles an update of the smartcontracts
// as well as inserts new decoded event logs in the database.
//
// Send indexer/handler.INDEXER command from network indexer sub service
// to execute this function.
func on_categorize(request message.Request, logger log.Logger, app_parameters ...interface{}) message.Reply {
	if app_parameters == nil || len(app_parameters) < 1 {
		return message.Fail("invalid parameters were given atleast database should be passed")
	}

	var categorize_parameters PushCategorization
	err := request.Parameters.ToInterface(&categorize_parameters)
	if err != nil {
		return message.Fail("invalid request parameters: " + err.Error())
	}
	if len(categorize_parameters.Smartcontracts) == 0 {
		return message.Fail("missing smartcontracts to update")
	}
	for _, sm := range categorize_parameters.Smartcontracts {
		if err := sm.Validate(); err != nil {
			return message.Fail("failed to validate smartcontract: " + err.Error())
		}
	}

	// the logs if given should match to the ones we already have
	for _, log := range categorize_parameters.Logs {
		if err := log.Validate(); err != nil {
			return message.Fail("failed to validate log: " + err.Error())
		}

		in_smartcontract := false
		for _, sm := range categorize_parameters.Smartcontracts {
			if log.SmartcontractKey == sm.SmartcontractKey {
				in_smartcontract = true
			}
		}
		if !in_smartcontract {
			return message.Fail("log to insert doesn't belong to list of smartcontracts: " + log.SmartcontractKey.ToString())
		}
	}

	// todo
	// cache the smartcontract list
	// then make sure that update time is not the same as
	// current smartcontract
	//
	// as well as make sure that log times are greater

	raw_smartcontracts, err := request.Parameters.GetKeyValueList("smartcontracts")
	if err != nil {
		return message.Fail("missing Smartcontracts")
	}

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

	db_con, ok := app_parameters[0].(*remote.ClientSocket)
	if !ok {
		return message.Fail("missing Manager in the parameters")
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
