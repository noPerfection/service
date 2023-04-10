package handler

import (
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/static/smartcontract"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"

	"github.com/blocklords/sds/app/command"
	"github.com/blocklords/sds/app/remote/message"
)

type SetSmartcontractRequest = smartcontract.Smartcontract
type SetSmartcontractReply = smartcontract.Smartcontract
type GetSmartcontractRequest = smartcontract_key.Key
type GetSmartcontractReply = smartcontract.Smartcontract

// Register a new smartcontract. It means we are adding smartcontract parameters into
// static_smartcontract.
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
		if err = smartcontract.SetInDatabase(db_con, &sm); err != nil {
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
