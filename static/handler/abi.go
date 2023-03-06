package handler

import (
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/db"
	"github.com/blocklords/sds/static/abi"
	"github.com/blocklords/sds/static/smartcontract"
	"github.com/charmbracelet/log"

	"github.com/blocklords/sds/app/remote/message"
)

// This function returns the ABI for the given smartcontract
//
//	Returning message.Reply {
//			params: {
//	     	"abi": []
//	     }
//	}
func AbiGet(con *db.Database, request message.Request, logger log.Logger) message.Reply {
	network_id, err := request.Parameters.GetString("network_id")
	if err != nil {
		return message.Fail(err.Error())
	}

	address, err := request.Parameters.GetString("address")
	if err != nil {
		return message.Fail(err.Error())
	}

	abi, err := abi.GetFromDatabase(con, network_id, address)
	if err != nil {
		return message.Fail(err.Error())
	}

	reply := message.Reply{
		Status:     "OK",
		Message:    "",
		Parameters: key_value.Empty().Set("abi", abi.ToString()).Set("abi_hash", abi.Id),
	}

	return reply
}

// Returns an abi by the smartcontract key.
func AbiGetBySmartcontractKey(db *db.Database, request message.Request, logger log.Logger) message.Reply {
	network_id, err := request.Parameters.GetString("network_id")
	if err != nil {
		return message.Fail(err.Error())
	}
	address, err := request.Parameters.GetString("address")
	if err != nil {
		return message.Fail(err.Error())
	}

	if !smartcontract.ExistInDatabase(db, network_id, address) {
		return message.Fail(`'` + network_id + `.` + address + `' smartcontract not registered in the SDS Static`)
	}

	smartcontract, err := smartcontract.GetFromDatabase(db, network_id, address)
	if err != nil {
		return message.Fail("failed to get smartcontract from database: " + err.Error())
	}

	abi, err := abi.GetFromDatabaseByAbiHash(db, smartcontract.AbiHash)
	if err != nil {
		return message.Fail("failed to get abi from database: " + err.Error())
	}

	return message.Reply{
		Status:     "OK",
		Message:    "",
		Parameters: key_value.Empty().Set("abi", abi.ToString()).Set("abi_hash", abi.Id),
	}
}

// inserts into the static database a new abi
//
//	Returning message.Reply {
//			params: {
//	     	"abi": [],
//	     	"abi_hash": "0x012345"
//	     }
//	}
func AbiRegister(dbCon *db.Database, request message.Request, logger log.Logger) message.Reply {
	abi_body, ok := request.Parameters["abi"]
	if !ok {
		return message.Fail("missing 'abi' parameter")
	}
	new_abi, err := abi.NewFromInterface(abi_body)
	if err != nil {
		return message.Fail(err.Error())
	}

	reply := message.Reply{
		Status:     "OK",
		Message:    "",
		Parameters: key_value.Empty().Set("abi", new_abi.ToString()).Set("abi_hash", new_abi.Id),
	}

	if abi.ExistInDatabase(dbCon, new_abi.Id) {
		return reply
	}

	save_err := abi.SetInDatabase(dbCon, new_abi)
	if save_err != nil {
		return message.Fail(err.Error())
	}

	return reply
}
