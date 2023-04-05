package handler

import (
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/db"
	"github.com/blocklords/sds/static/configuration"

	"github.com/blocklords/sds/app/command"
	"github.com/blocklords/sds/app/remote/message"
)

type GetConfigurationRequest = configuration.Configuration
type SetConfigurationRequest = configuration.Configuration
type GetConfigurationReply = configuration.Configuration
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
	if err := conf.Validate(); err != nil {
		return message.Fail("validation: " + err.Error())
	}

	conf_list := parameters[3].(*key_value.List)
	conf_key := conf.Topic.ToString(conf.Topic.Level())
	_, err = conf_list.Get(conf_key)
	if err != nil {
		return message.Fail("failed to get smartcontract: " + err.Error())
	}

	err = conf_list.Add(conf_key, &conf)
	if err != nil {
		return message.Fail("failed to add abi to abi list: " + err.Error())
	}

	if err = configuration.SetInDatabase(db_con, &conf); err != nil {
		return message.Fail("Configuration saving in the database failed: " + err.Error())
	}

	var reply GetConfigurationReply = conf
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("failed to reply")
	}

	return reply_message
}

// Returns configuration and smartcontract information related to the configuration
func ConfigurationGet(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	var conf GetConfigurationRequest
	err := request.Parameters.ToInterface(&conf)
	if err != nil {
		return message.Fail("failed to parse data")
	}
	if err := conf.Topic.Validate(); err != nil {
		return message.Fail("invalid topic: " + err.Error())
	}

	conf_list := parameters[3].(*key_value.List)
	conf_key := conf.Topic.ToString(conf.Topic.Level())
	conf_raw, err := conf_list.Get(conf_key)
	if err != nil {
		return message.Fail("failed to get configuration: " + err.Error())
	}

	var reply GetConfigurationReply = conf_raw.(configuration.Configuration)
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("failed to reply")
	}

	return reply_message
}
