package handler

import (
	"github.com/blocklords/sds/app/command"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"
	blockchain_command "github.com/blocklords/sds/blockchain/handler"
	blockchain_inproc "github.com/blocklords/sds/blockchain/inproc"
	"github.com/blocklords/sds/blockchain/network"
	"github.com/blocklords/sds/categorizer/event"
	"github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/common/data_type/database"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
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
type SetSmartcontractReply struct{}

type GetSmartcontractsRequest struct{}
type GetSmartcontractsReply struct {
	Smartcontracts []smartcontract.Smartcontract `json:"smartcontracts"`
}
type PushCategorization struct {
	Smartcontracts []smartcontract.Smartcontract `json:"smartcontracts"`
	Logs           []event.Log                   `json:"logs"`
}
type CategorizationReply key_value.KeyValue

// return a categorized smartcontract parameters by network id and smartcontract address
func GetSmartcontract(request message.Request, _ log.Logger, app_parameters ...interface{}) message.Reply {
	if len(app_parameters) < 3 {
		return message.Fail("missing database client socket in the app parameters")
	}
	key, err := smartcontract_key.NewFromKeyValue(request.Parameters)
	if err != nil {
		return message.Fail("smartcontract_key.NewFromKeyValue: " + err.Error())
	}

	sm := smartcontract.Smartcontract{SmartcontractKey: key}

	db_con, ok := app_parameters[0].(*remote.ClientSocket)
	if !ok {
		return message.Fail("missing database client socket in app parameters")
	}

	var crud database.Crud = &sm

	err = crud.Select(db_con)
	if err != nil {
		return message.Fail("smartcontract.Get: " + err.Error())
	}

	reply := GetSmartcontractReply{
		Smartcontract: sm,
	}

	reply_message, err := command.Reply(reply)
	if err != nil {
		return message.Fail("parse reply: " + err.Error())
	}

	return reply_message

}

// returns all categorized smartcontracts
func GetSmartcontracts(_ message.Request, _ log.Logger, app_parameters ...interface{}) message.Reply {
	if len(app_parameters) < 3 {
		return message.Fail("missing database client socket in the app parameters")
	}

	db_con, ok := app_parameters[0].(*remote.ClientSocket)
	if !ok {
		return message.Fail("missing database client socket in app parameters")
	}
	var smartcontracts []smartcontract.Smartcontract
	var crud database.Crud = &smartcontract.Smartcontract{}
	err := crud.SelectAll(db_con, &smartcontracts)
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
func SetSmartcontract(request message.Request, _ log.Logger, app_parameters ...interface{}) message.Reply {
	if len(app_parameters) < 3 {
		return message.Fail("missing database client socket, network sockets and networks in the app parameters")
	}

	db_con, ok := app_parameters[0].(*remote.ClientSocket)
	if !ok {
		return message.Fail("missing database client socket in app parameters")
	}

	var request_parameters SetSmartcontractRequest
	err := request.Parameters.ToInterface(&request_parameters)
	if err != nil {
		return message.Fail("parsing request parameters: " + err.Error())
	}

	if err := request_parameters.Smartcontract.Validate(); err != nil {
		return message.Fail("validating request parameters: " + err.Error())
	}

	var crud database.Crud = &request_parameters.Smartcontract

	if crud.Exist(db_con) {
		return message.Fail("the smartcontract already in SDS Categorizer")
	}

	saveErr := crud.Insert(db_con)
	if saveErr != nil {
		return message.Fail("database: " + saveErr.Error())
	}

	networks, ok := app_parameters[2].(network.Networks)
	if !ok {
		return message.Fail("no networks were given")
	}
	if !networks.Exist(request_parameters.Smartcontract.SmartcontractKey.NetworkId) {
		return message.Fail("network data not found for network id: " + request_parameters.Smartcontract.SmartcontractKey.NetworkId)
	}
	network, err := networks.Get(request_parameters.Smartcontract.SmartcontractKey.NetworkId)
	if err != nil {
		return message.Fail("networks.Get: " + err.Error())
	}

	network_sockets, ok := app_parameters[1].(key_value.KeyValue)
	if !ok {
		return message.Fail("no network sockets in the app parameters")
	}

	client_socket, ok := network_sockets[network.Type.String()].(*remote.ClientSocket)
	if !ok {
		return message.Fail("no network client for " + network.Type.String())
	}

	url := blockchain_inproc.CategorizerEndpoint(network.Id)
	categorizer_service, err := service.InprocessFromUrl(url)
	if err != nil {
		return message.Fail("blockchain_inproc.CategorizerEndpoint(network.Id): " + err.Error())
	}

	new_sm_request := blockchain_command.PushNewSmartcontracts{
		Smartcontracts: []smartcontract.Smartcontract{request_parameters.Smartcontract},
	}
	var new_sm_reply key_value.KeyValue
	err = blockchain_command.NEW_CATEGORIZED_SMARTCONTRACTS.RequestRouter(client_socket, categorizer_service, new_sm_request, &new_sm_reply)
	if err != nil {
		return message.Fail("failed to send to blockchain package: " + err.Error())
	}

	reply := SetSmartcontractReply{}
	reply_message, err := command.Reply(reply)
	if err != nil {
		return message.Fail("parse reply: " + err.Error())
	}

	return reply_message
}
