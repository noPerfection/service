package handler

import (
	"github.com/blocklords/gosds/db"
	"github.com/blocklords/gosds/static/abi"
	"github.com/blocklords/gosds/static/smartcontract"

	"github.com/blocklords/gosds/app/remote/message"
)

// This function returns the ABI for the given smartcontract
//
//	Returning message.Reply {
//			params: {
//	     	"abi": []
//	     }
//	}
func AbiGet(con *db.Database, request message.Request) message.Reply {
	network_id, err := message.GetString(request.Parameters, "network_id")
	if err != nil {
		return message.Fail(err.Error())
	}

	address, err := message.GetString(request.Parameters, "address")
	if err != nil {
		return message.Fail(err.Error())
	}

	abi, err := abi.GetFromDatabase(con, network_id, address)
	if err != nil {
		return message.Fail(err.Error())
	}

	reply := message.Reply{
		Status:  "OK",
		Message: "",
		Params:  abi.ToJSON(),
	}

	return reply
}

// Returns an abi by the smartcontract key.
func AbiGetBySmartcontractKey(db *db.Database, request message.Request) message.Reply {
	network_id, err := message.GetString(request.Parameters, "network_id")
	if err != nil {
		return message.Fail(err.Error())
	}
	address, err := message.GetString(request.Parameters, "address")
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

	abi := abi.GetFromDatabaseByAbiHash(db, smartcontract.AbiHash)

	return message.Reply{
		Status:  "OK",
		Message: "",
		Params:  abi.ToJSON(),
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
func AbiRegister(dbCon *db.Database, request message.Request) message.Reply {
	abi_body, ok := request.Parameters["abi"]
	if !ok {
		return message.Fail("missing 'abi' parameter")
	}
	new_abi, err := abi.New(abi_body)

	if err != nil {
		return message.Fail(err.Error())
	}

	reply := message.Reply{
		Status:  "OK",
		Message: "",
		Params:  new_abi.ToJSON(),
	}

	if abi.ExistInDatabase(dbCon, new_abi.AbiHash) {
		return reply
	}

	save_err := abi.SetInDatabase(dbCon, new_abi)
	if save_err != nil {
		return message.Fail(err.Error())
	}

	return reply
}
