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
type GetAbiReply = abi.Abi

type SetAbiRequest struct {
	Body interface{} `json:"body"`
}
type SetAbiReply = abi.Abi

// Returns an abi by abi id
func AbiGet(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	var req_parameters GetAbiRequest
	err := request.Parameters.ToInterface(&req_parameters)
	if err != nil {
		return message.Fail("request.Parameters -> Command Parameter: " + err.Error())
	}
	if len(req_parameters.Id) == 0 {
		return message.Fail("missing abi id")
	}

	abi_list, ok := parameters[1].(*key_value.List)
	if !ok {
		return message.Fail("missing abi list")
	}
	abi_raw, err := abi_list.Get(req_parameters.Id)
	if err != nil {
		return message.Fail("abi_list.Get: " + err.Error())
	}

	reply_message, err := command.Reply(abi_raw)
	if err != nil {
		return message.Fail("failed to reply")
	}

	return reply_message
}

func AbiRegister(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	var request_parameters SetAbiRequest
	err := request.Parameters.ToInterface(&request_parameters)
	if err != nil {
		return message.Fail("failed to parse data")
	}

	if request_parameters.Body == nil {
		return message.Fail("missing body")
	}

	new_abi, err := abi.NewFromInterface(request_parameters.Body)
	if err != nil {
		return message.Fail("abi.NewFromInterface: " + err.Error())
	}
	if len(new_abi.Bytes) == 0 {
		return message.Fail("body is empty")
	}

	reply_message, err := command.Reply(new_abi)
	if err != nil {
		return message.Fail("failed to reply")
	}

	abi_list, ok := parameters[1].(*key_value.List)
	if !ok {
		return message.Fail("missing abi list")
	}
	_, err = abi_list.Get(new_abi.Id)
	if err == nil {
		return message.Fail("abi registered already for")
	}

	err = abi_list.Add(new_abi.Id, new_abi)
	if err != nil {
		return message.Fail("failed to add abi to abi list: " + err.Error())
	}

	db_con, ok := parameters[0].(*db.Database)
	if ok {
		save_err := abi.SetInDatabase(db_con, new_abi)
		if save_err != nil {
			return message.Fail("database error:" + err.Error())
		}
	}

	return reply_message
}
