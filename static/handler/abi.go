package handler

import (
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/db"
	"github.com/blocklords/sds/static/abi"
	"github.com/blocklords/sds/static/smartcontract"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/charmbracelet/log"

	"github.com/blocklords/sds/app/remote/message"
)

// Returns an abi by the smartcontract key.
func AbiGetBySmartcontractKey(request message.Request, logger log.Logger, parameters ...interface{}) message.Reply {
	db_con := parameters[0].(*db.Database)

	key, err := smartcontract_key.NewFromKeyValue(request.Parameters)
	if err != nil {
		return message.Fail("smartcontract_key from parameter: " + err.Error())
	}

	smartcontract, err := smartcontract.GetFromDatabase(db_con, key)
	if err != nil {
		return message.Fail("failed to get smartcontract from database: " + err.Error())
	}

	abi, err := abi.GetFromDatabaseByAbiId(db_con, smartcontract.AbiId)
	if err != nil {
		return message.Fail("failed to get abi from database: " + err.Error())
	}

	return message.Reply{
		Status:     "OK",
		Message:    "",
		Parameters: key_value.Empty().Set("body", abi.ToString()).Set("abi_id", abi.Id),
	}
}

// inserts into the static database a new abi
//
//	Returning message.Reply {
//			params: {
//	     	"body": [],
//	     	"abi_id": "0x012345"
//	     }
//	}
func AbiRegister(request message.Request, logger log.Logger, parameters ...interface{}) message.Reply {
	db_con := parameters[0].(*db.Database)

	abi_body, ok := request.Parameters["body"]
	if !ok {
		return message.Fail("missing 'body' parameter")
	}
	new_abi, err := abi.NewFromInterface(abi_body)
	if err != nil {
		return message.Fail(err.Error())
	}

	reply := message.Reply{
		Status:     "OK",
		Message:    "",
		Parameters: key_value.Empty().Set("body", new_abi.ToString()).Set("abi_id", new_abi.Id),
	}

	if abi.ExistInDatabase(db_con, new_abi.Id) {
		return reply
	}

	save_err := abi.SetInDatabase(db_con, new_abi)
	if save_err != nil {
		return message.Fail(err.Error())
	}

	return reply
}
