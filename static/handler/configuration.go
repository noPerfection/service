package handler

import (
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/db"
	"github.com/blocklords/sds/static/configuration"
	"github.com/blocklords/sds/static/smartcontract"

	"github.com/blocklords/sds/app/remote/command"
	"github.com/blocklords/sds/app/remote/message"
)

type GetConfigurationRequest = configuration.Configuration
type SetConfigurationRequest = configuration.Configuration
type GetConfigurationReply struct {
	Configuration configuration.Configuration `json:"configuration"`
	Smartcontract smartcontract.Smartcontract `json:"smartcontract"`
}
type SetConfigurationReply = configuration.Configuration

// Register a new smartcontract in the configuration.
// It requires smartcontract address. First call smartcontract_register command.
func ConfigurationRegister(request message.Request, logger log.Logger, parameters ...interface{}) message.Reply {
	db_con := parameters[0].(*db.Database)

	var conf GetConfigurationRequest
	err := request.Parameters.ToInterface(&conf)
	if err != nil {
		return message.Fail("failed to parse data")
	}

	if configuration.ExistInDatabase(db_con, &conf) {
		return message.Fail("Smartcontract found in the config")
	}

	if err = configuration.SetInDatabase(db_con, &conf); err != nil {
		return message.Fail("Configuration saving in the database failed: " + err.Error())
	}

	reply := GetConfigurationReply{
		Configuration: conf,
	}
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("failed to reply")
	}

	return reply_message
}

// Returns configuration and smartcontract information related to the configuration
func ConfigurationGet(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	db_con := parameters[0].(*db.Database)

	var conf GetConfigurationRequest
	err := request.Parameters.ToInterface(&conf)
	if err != nil {
		return message.Fail("failed to parse data")
	}

	if !configuration.ExistInDatabase(db_con, &conf) {
		return message.Fail("Configuration not registered in the database")
	}

	err = configuration.LoadDatabaseParts(db_con, &conf)
	if err != nil {
		return message.Fail("Configuration loading in the database failed: " + err.Error())
	}

	s, getErr := smartcontract.GetFromDatabase(db_con, smartcontract_key.New(conf.Topic.NetworkId, conf.Address))
	if getErr != nil {
		return message.Fail("Failed to get smartcontract from database: " + getErr.Error())
	}

	reply := GetConfigurationReply{
		Configuration: conf,
		Smartcontract: *s,
	}
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("failed to reply")
	}

	return reply_message
}
