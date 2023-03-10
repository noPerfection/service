package handler

import (
	"github.com/charmbracelet/log"

	"github.com/blocklords/sds/app/remote/message"
	blockchain_process "github.com/blocklords/sds/blockchain/inproc"
	"github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/common/data_type"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/smartcontract_key"

	"github.com/blocklords/sds/db"
)

// return a categorized smartcontract parameters by network id and smartcontract address
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

// Register a new smartcontract to categorizer.
func SetSmartcontract(request message.Request, logger log.Logger, parameters ...interface{}) message.Reply {
	db_con := parameters[0].(*db.Database)

	kv, err := request.Parameters.GetKeyValue("smartcontract")
	if err != nil {
		return message.Fail("missing 'smartcontract' parameter")
	}

	sm, err := smartcontract.New(kv)
	if err != nil {
		return message.Fail("request parameter -> smartcontract.New: " + err.Error())
	}

	if smartcontract.Exists(db_con, sm.SmartcontractKey) {
		return message.Fail("the smartcontract already in SDS Categorizer")
	}

	saveErr := smartcontract.Save(db_con, sm)
	if saveErr != nil {
		return message.Fail("database: " + saveErr.Error())
	}

	pusher, err := blockchain_process.CategorizerManagerSocket(sm.SmartcontractKey.NetworkId)
	if err != nil {
		return message.Fail("inproc: " + err.Error())
	}
	defer pusher.Close()

	smartcontracts := []*smartcontract.Smartcontract{sm}

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

	reply := message.Reply{
		Status:     "OK",
		Message:    "",
		Parameters: key_value.Empty().Set("smartcontract", sm),
	}

	return reply
}
