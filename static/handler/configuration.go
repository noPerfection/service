package handler

import (
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/db"
	"github.com/blocklords/sds/static/configuration"
	"github.com/blocklords/sds/static/smartcontract"
	"github.com/blocklords/sds/static/smartcontract/key"
	"github.com/charmbracelet/log"

	"github.com/blocklords/sds/app/remote/message"
)

// Register a new smartcontract in the configuration.
// It requires smartcontract address. First call smartcontract_register command.
func ConfigurationRegister(request message.Request, logger log.Logger, parameters ...interface{}) message.Reply {
	db_con := parameters[0].(*db.Database)

	conf, err := configuration.New(request.Parameters)
	if err != nil {
		return message.Fail(err.Error())
	}

	if configuration.ExistInDatabase(db_con, conf) {
		return message.Fail("Smartcontract found in the config")
	}

	if err = configuration.SetInDatabase(db_con, conf); err != nil {
		return message.Fail("Configuration saving in the database failed: " + err.Error())
	}

	return message.Reply{
		Status:     "OK",
		Message:    "",
		Parameters: key_value.Empty().Set("configuration", conf),
	}
}

// Returns configuration and smartcontract information related to the configuration
func ConfigurationGet(request message.Request, logger log.Logger, parameters ...interface{}) message.Reply {
	db_con := parameters[0].(*db.Database)

	conf, err := configuration.New(request.Parameters)
	if err != nil {
		return message.Fail(err.Error())
	}

	if !configuration.ExistInDatabase(db_con, conf) {
		return message.Fail("Configuration not registered in the database")
	}

	err = configuration.LoadDatabaseParts(db_con, conf)
	if err != nil {
		return message.Fail("Configuration loading in the database failed: " + err.Error())
	}

	s, getErr := smartcontract.GetFromDatabase(db_con, key.New(conf.NetworkId, conf.Address()))
	if getErr != nil {
		return message.Fail("Failed to get smartcontract from database: " + getErr.Error())
	}

	reply := message.Reply{
		Status:     "OK",
		Message:    "",
		Parameters: key_value.Empty().Set("configuration", conf).Set("smartcontract", s),
	}
	return reply
}
