package handler

import (
	"github.com/blocklords/sds/common/data_type/database"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/topic"
	"github.com/blocklords/sds/service/log"
	"github.com/blocklords/sds/service/remote"
	"github.com/blocklords/sds/storage/configuration"

	"github.com/blocklords/sds/service/communication/command"
	"github.com/blocklords/sds/service/communication/message"
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

	_, err = conf_list.Get(conf.Topic)
	if err == nil {
		return message.Fail("configuration already added")
	}

	err = conf_list.Add(conf.Topic, &conf)
	if err != nil {
		return message.Fail("failed to add abi to abi list: " + err.Error())
	}

	db_con, ok := parameters[0].(*remote.ClientSocket)
	if ok {
		var crud database.Crud = &conf
		if err = crud.Insert(db_con); err != nil {
			return message.Fail("Configuration saving in the database failed: " + err.Error())
		}
	}

	var reply = conf
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

	conf_raw, err := conf_list.Get(conf_topic)
	if err != nil {
		return message.Fail("failed to get configuration or not found: " + err.Error())
	}

	var reply = *conf_raw.(*configuration.Configuration)
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("failed to reply")
	}

	return reply_message
}
