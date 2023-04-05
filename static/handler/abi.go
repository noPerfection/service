package handler

import (
	"github.com/blocklords/sds/app/command"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/db"
	"github.com/blocklords/sds/static/abi"

	"github.com/blocklords/sds/app/remote/message"
)

type GetAbiRequest struct {
	Id string `json:"abi_id"`
}

type SetAbiRequest struct {
	Body interface{} `json:"body"`
}

type GetAbiReply = abi.Abi
type SetAbiReply = GetAbiReply

// Returns an abi by abi id
func AbiGet(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	var req_parameters GetAbiRequest
	err := request.Parameters.ToInterface(&req_parameters)
	if err != nil {
		return message.Fail("failed to parse data")
	}
	if len(req_parameters.Id) == 0 {
		return message.Fail("missing abi id")
	}

	abi_list := parameters[1].(*key_value.List)
	abi_raw, err := abi_list.Get(req_parameters.Id)
	if err != nil {
		return message.Fail("failed to get abi: " + err.Error())
	}

	reply_message, err := command.Reply(abi_raw)
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

	abi_list := parameters[1].(*key_value.List)
	_, err = abi_list.Get(new_abi.Id)
	if err != nil {
		return message.Fail("failed to get abi: " + err.Error())
	}

	err = abi_list.Add(new_abi.Id, new_abi)
	if err != nil {
		return message.Fail("failed to add abi to abi list: " + err.Error())
	}

	save_err := abi.SetInDatabase(db_con, new_abi)
	if save_err != nil {
		return message.Fail("database error:" + err.Error())
	}

	return reply_message
}
