package handler

import (
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/categorizer/event"
	"github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/common/data_type/key_value"
)

func on_new_smartcontracts(request message.Request, logger log.Logger, parameters ...interface{}) message.Reply {
	if parameters == nil || len(parameters) < 1 {
		return message.Fail("invalid parameters were given atleast database should be passed")
	}

	db_con, ok := parameters[0].(*remote.ClientSocket)
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
		err := smartcontract.SaveBlockParameters(db_con, sm)
		if err != nil {
			return message.Fail("smartcontract.SaveBlockParameters: " + err.Error())
		}
	}

	for _, l := range logs {
		err := event.Save(db_con, l)
		if err != nil {
			return message.Fail("event.Save: " + err.Error())
		}
	}

	return message.Reply{
		Status:     message.OK,
		Message:    "",
		Parameters: key_value.Empty(),
	}
}
