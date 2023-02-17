package handler

import (
	"github.com/blocklords/gosds/categorizer/smartcontract"
	"github.com/blocklords/gosds/db"

	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/common/data_type"
	"github.com/blocklords/gosds/common/data_type/key_value"
)

// return a categorizer block by network id and smartcontract address
func GetSmartcontract(db *db.Database, request message.Request) message.Reply {
	network_id, err := request.Parameters.GetString("network_id")
	if err != nil {
		return message.Fail(err.Error())
	}
	address, err := request.Parameters.GetString("address")
	if err != nil {
		return message.Fail(err.Error())
	}

	sm, err := smartcontract.Get(db, network_id, address)

	if err != nil {
		return message.Fail("the smartcontract not found in the SDS Categorizer")
	}

	reply := message.Reply{
		Status: "OK",
		Parameters: key_value.New(map[string]interface{}{
			"smartcontract": sm.ToJSON(),
		}),
	}

	return reply

}

// returns all smartcontract categorized smartcontracts
func GetSmartcontracts(db *db.Database, _ message.Request) message.Reply {
	smartcontracts, err := smartcontract.GetAll(db)
	if err != nil {
		return message.Fail("the database error " + err.Error())
	}

	reply := message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"smartcontracts": data_type.ToMapList(smartcontracts),
		}),
	}

	return reply
}
