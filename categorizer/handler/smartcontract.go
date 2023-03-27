package handler

import (
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote/command"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/common/smartcontract_key"

	"github.com/blocklords/sds/db"

	zmq "github.com/pebbe/zmq4"
)

type GetSmartcontractRequest struct {
	Key smartcontract_key.Key
}
type GetSmartcontractReply struct {
	Smartcontract smartcontract.Smartcontract `json:"smartcontract"`
}

type SetSmartcontractRequest struct {
	Smartcontract smartcontract.Smartcontract `json:"smartcontract"`
}
type SetSmartcontractsReply struct{}

type GetSmartcontractsRequest struct{}
type GetSmartcontractsReply struct {
	Smartcontracts []smartcontract.Smartcontract `json:"smartcontracts"`
}

// return a categorized smartcontract parameters by network id and smartcontract address
func GetSmartcontract(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	db := parameters[0].(*db.Database)

	key, err := smartcontract_key.NewFromKeyValue(request.Parameters)
	if err != nil {
		return message.Fail("smartcontract_key.NewFromKeyValue: " + err.Error())
	}

	sm, err := smartcontract.Get(db, key)

	if err != nil {
		return message.Fail("smartcontract.Get: " + err.Error())
	}

	reply := GetSmartcontractReply{
		Smartcontract: *sm,
	}

	reply_message, err := command.Reply(reply)
	if err != nil {
		return message.Fail("parse reply: " + err.Error())
	}

	return reply_message

}

// returns all smartcontract categorized smartcontracts
func GetSmartcontracts(_ message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	db := parameters[0].(*db.Database)
	smartcontracts, err := smartcontract.GetAll(db)
	if err != nil {
		return message.Fail("the database error " + err.Error())
	}

	reply := GetSmartcontractsReply{
		Smartcontracts: smartcontracts,
	}

	reply_message, err := command.Reply(reply)
	if err != nil {
		return message.Fail("parse reply: " + err.Error())
	}

	return reply_message
}

// Register a new smartcontract to categorizer.
func SetSmartcontract(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	db_con := parameters[0].(*db.Database)

	var request_parameters SetSmartcontractRequest
	err := request.Parameters.ToInterface(&request_parameters)
	if err != nil {
		return message.Fail("parsing request parameters: " + err.Error())
	}

	if smartcontract.Exists(db_con, request_parameters.Smartcontract.SmartcontractKey) {
		return message.Fail("the smartcontract already in SDS Categorizer")
	}

	saveErr := smartcontract.Save(db_con, &request_parameters.Smartcontract)
	if saveErr != nil {
		return message.Fail("database: " + saveErr.Error())
	}

	pushers := parameters[1].(map[string]*zmq.Socket)
	pusher, ok := pushers[request_parameters.Smartcontract.SmartcontractKey.NetworkId]
	if !ok {
		return message.Fail("no blockchain package for network id")
	}

	smartcontracts := []*smartcontract.Smartcontract{&request_parameters.Smartcontract}

	push := message.Request{
		Command: "new-smartcontracts",
		Parameters: map[string]interface{}{
			"smartcontracts": smartcontracts,
		},
	}
	request_string, _ := push.ToString()

	_, err = pusher.SendMessage(request_string)
	if err != nil {
		return message.Fail("send: " + err.Error())
	}

	reply := SetSmartcontractsReply{}
	reply_message, err := command.Reply(reply)
	if err != nil {
		return message.Fail("parse reply: " + err.Error())
	}

	return reply_message
}
