package handler

import (
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote/command"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/db"
	"github.com/blocklords/sds/static/abi"
	"github.com/blocklords/sds/static/smartcontract"

	"github.com/blocklords/sds/app/remote/message"
)

type GetAbiRequest = smartcontract_key.Key

type SetAbiRequest struct {
	Body interface{} `json:"body"`
}

type GetAbiReply = abi.Abi
type SetAbiReply = GetAbiReply

// Returns an abi by the smartcontract key.
func AbiGetBySmartcontractKey(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	db_con := parameters[0].(*db.Database)

	var key GetAbiRequest
	err := request.Parameters.ToInterface(&key)
	if err != nil {
		return message.Fail("failed to parse data")
	}

	smartcontract, err := smartcontract.GetFromDatabase(db_con, key)
	if err != nil {
		return message.Fail("failed to get smartcontract from database: " + err.Error())
	}

	abi, err := abi.GetFromDatabaseByAbiId(db_con, smartcontract.AbiId)
	if err != nil {
		return message.Fail("failed to get abi from database: " + err.Error())
	}

	reply_message, err := command.Reply(abi)
	if err != nil {
		return message.Fail("failed to reply")
	}

	return reply_message
}

func AbiRegister(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	db_con := parameters[0].(*db.Database)

	var request_parameters SetAbiRequest
	err := request.Parameters.ToInterface(&request_parameters)
	if err != nil {
		return message.Fail("failed to parse data")
	}

	new_abi, err := abi.NewFromInterface(request_parameters.Body)
	if err != nil {
		return message.Fail(err.Error())
	}

	reply_message, err := command.Reply(new_abi)
	if err != nil {
		return message.Fail("failed to reply")
	}

	if abi.ExistInDatabase(db_con, new_abi.Id) {
		return reply_message
	}

	save_err := abi.SetInDatabase(db_con, new_abi)
	if save_err != nil {
		return message.Fail(err.Error())
	}

	return reply_message
}
