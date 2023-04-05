package handler

import (
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/topic"
	"github.com/blocklords/sds/db"
	"github.com/blocklords/sds/static/configuration"

	"github.com/blocklords/sds/app/command"
	"github.com/blocklords/sds/app/remote/message"
)

type GetConfigurationRequest = topic.Topic
type GetConfigurationReply = configuration.Configuration

type SetConfigurationRequest = configuration.Configuration
type SetConfigurationReply = configuration.Configuration

// Register a new smartcontract in the configuration.
// It requires smartcontract address. First call smartcontract_register command.
func ConfigurationRegister(request message.Request, logger log.Logger, parameters ...interface{}) message.Reply {
	if len(parameters) < 4 {
		return message.Fail("missing configuration list")
	}
	conf_list, ok := parameters[3].(*key_value.List)
	if !ok {
		return message.Fail("configuration list expected")
	}

	var conf SetConfigurationRequest
	err := request.Parameters.ToInterface(&conf)
	if err != nil {
		return message.Fail("failed to parse data")
	}
	if err := conf.Validate(); err != nil {
		return message.Fail("validation: " + err.Error())
	}

	conf_key := conf.Topic.ToString(conf.Topic.Level())
	_, err = conf_list.Get(conf_key)
	if err != nil {
		return message.Fail("failed to get smartcontract: " + err.Error())
	}

	err = conf_list.Add(conf_key, &conf)
	if err != nil {
		return message.Fail("failed to add abi to abi list: " + err.Error())
	}

	db_con, ok := parameters[0].(*db.Database)
	if ok {
		if err = configuration.SetInDatabase(db_con, &conf); err != nil {
			return message.Fail("Configuration saving in the database failed: " + err.Error())
		}
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
	if len(parameters) < 4 {
		return message.Fail("missing configuration list")
	}
	conf_list, ok := parameters[3].(*key_value.List)
	if !ok {
		return message.Fail("configuration list expected")
	}

	var conf_topic GetConfigurationRequest
	err := request.Parameters.ToInterface(&conf_topic)
	if err != nil {
		return message.Fail("failed to parse data")
	}
	if err := conf_topic.Validate(); err != nil {
		return message.Fail("invalid topic: " + err.Error())
	}
	if conf_topic.Level() != topic.SMARTCONTRACT_LEVEL {
		return message.Fail("topic level is not at SMARTCONTRACT LEVEL")
	}

	conf_key := conf_topic.ToString(topic.SMARTCONTRACT_LEVEL)
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
