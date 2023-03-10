package handler

import (
	"github.com/charmbracelet/log"

	"github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/db"

	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/common/data_type"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"
)

// return a categorizer block by network id and smartcontract address
func GetSmartcontract(request message.Request, logger log.Logger, parameters ...interface{}) message.Reply {
	db := parameters[0].(*db.Database)

	key, err := smartcontract_key.NewFromKeyValue(request.Parameters)
	if err != nil {
		return message.Fail("smartcontract_key.NewFromKeyValue: " + err.Error())
	}

	sm, err := smartcontract.Get(db, key)

	if err != nil {
		return message.Fail("smartcontract.Get: " + err.Error())
	}

	reply := message.Reply{
		Status:     "OK",
		Parameters: key_value.Empty().Set("smartcontract", sm),
	}

	return reply

}

// returns all smartcontract categorized smartcontracts
func GetSmartcontracts(_ message.Request, logger log.Logger, parameters ...interface{}) message.Reply {
	db := parameters[0].(*db.Database)
	smartcontracts, err := smartcontract.GetAll(db)
	if err != nil {
		return message.Fail("the database error " + err.Error())
	}

	reply := message.Reply{
		Status:     "OK",
		Message:    "",
		Parameters: key_value.Empty().Set("smartcontracts", data_type.ToMapList(smartcontracts)),
	}

	return reply
}
