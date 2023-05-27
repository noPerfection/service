package handler

import (
	"github.com/blocklords/sds/service/log"
	"github.com/blocklords/sds/service/remote"
	"github.com/blocklords/sds/storage/smartcontract"

	"github.com/blocklords/sds/common/data_type/database"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"

	"github.com/blocklords/sds/service/communication/command"
	"github.com/blocklords/sds/service/communication/message"
)

type SetSmartcontractRequest = smartcontract.Smartcontract
type SetSmartcontractReply = smartcontract.Smartcontract
type GetSmartcontractRequest = smartcontract_key.Key
type GetSmartcontractReply = smartcontract.Smartcontract

// Register a new smartcontract. It means we are adding smartcontract parameters into
// smartcontract database table.
// Requires abi_id parameter. First call abi_register method first.
func SmartcontractRegister(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	if len(parameters) < 3 {
		return message.Fail("missing smartcontract list")
	}

	var sm SetSmartcontractRequest
	err := request.Parameters.ToInterface(&sm)
	if err != nil {
		return message.Fail("failed to parse data")
	}
	if err := sm.Validate(); err != nil {
		return message.Fail("failed to validate: " + err.Error())
	}

	var reply = sm
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("failed to reply")
	}

	sm_list, ok := parameters[2].(*key_value.List)
	if !ok {
		return message.Fail("no smartcontract list")
	}
	_, err = sm_list.Get(sm.SmartcontractKey)
	if err == nil {
		return message.Fail("smartcontract already registered")
	}

	err = sm_list.Add(sm.SmartcontractKey, &sm)
	if err != nil {
		return message.Fail("failed to add abi to abi list: " + err.Error())
	}

	db_con, ok := parameters[0].(*remote.ClientSocket)
	if ok {
		var crud database.Crud = &sm
		if err = crud.Insert(db_con); err != nil {
			return message.Fail("Smartcontract saving in the database failed: " + err.Error())
		}
	}

	return reply_message
}

// Returns configuration and smartcontract information related to the configuration
func SmartcontractGet(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	if len(parameters) < 3 {
		return message.Fail("missing smartcontract list")
	}

	var key GetSmartcontractRequest
	err := request.Parameters.ToInterface(&key)
	if err != nil {
		return message.Fail("failed to parse data")
	}
	if err := key.Validate(); err != nil {
		return message.Fail("key.Validate: " + err.Error())
	}

	sm_list, ok := parameters[2].(*key_value.List)
	if !ok {
		return message.Fail("no smartcontract list")
	}
	sm_raw, err := sm_list.Get(key)
	if err != nil {
		return message.Fail("failed to get smartcontract: " + err.Error())
	}

	var reply = *sm_raw.(*smartcontract.Smartcontract)
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("failed to reply")
	}

	return reply_message
}
