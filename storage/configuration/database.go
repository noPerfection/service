package configuration

import (
	"fmt"

	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/topic"
	"github.com/blocklords/sds/database/handler"
	"github.com/blocklords/sds/service/remote"
)

// Inserts the configuration into the database
//
// It doesn't validates the configuration.
// Call conf.Validate() before calling this
//
// Implements common/data_type/database.Crud interface
func (conf *Configuration) Insert(db *remote.ClientSocket) error {
	request := handler.DatabaseQueryRequest{
		Fields:    []string{"organization", "project", "network_id", "group_name", "smartcontract_name", "address"},
		Tables:    []string{"configuration"},
		Arguments: []interface{}{conf.Topic.Organization, conf.Topic.Project, conf.Topic.NetworkId, conf.Topic.Group, conf.Topic.Smartcontract, conf.Address},
	}
	var reply handler.InsertReply

	err := handler.INSERT.Request(db, request, &reply)
	if err != nil {
		return fmt.Errorf("handler.INSERT.Request: %w", err)
	}
	return nil
}

// SelectAll configurations from database
//
// Implements common/data_type/database.Crud interface
func (conf *Configuration) SelectAll(db *remote.ClientSocket, return_values interface{}) error {
	confs, ok := return_values.(*[]*Configuration)
	if !ok {
		return fmt.Errorf("return_values.(*[]*Configuration)")
	}

	request := handler.DatabaseQueryRequest{
		Fields: []string{
			"organization as o",
			"project as p",
			"network_id as n",
			"group_name as g",
			"smartcontract_name as s",
			"address",
		},
		Tables: []string{"configuration"},
	}
	var reply handler.SelectAllReply

	err := handler.SELECT_ALL.Request(db, request, &reply)
	if err != nil {
		return fmt.Errorf("handler.SELECT_ALL.Request: %w", err)
	}

	*confs = make([]*Configuration, len(reply.Rows))

	// Loop through rows, using Scan to assign column data to struct fields.
	for i, raw := range reply.Rows {
		conf_topic, err := topic.ParseJSON(raw)
		if err != nil {
			return fmt.Errorf("parsing topic parameters from database result failed: %w", err)
		}
		address, err := raw.GetString("address")
		if err != nil {
			return fmt.Errorf("parsing address parameter from database result failed: %w", err)
		}
		conf, err := NewFromTopic(*conf_topic, address)
		if err != nil {
			return fmt.Errorf("NewFromTopic: %w", err)
		}
		(*confs)[i] = conf
	}
	return_values = confs

	return err
}

// Not implemented common/data_type/database.Crud interface
//
// Returns an error
func (conf *Configuration) Select(_ *remote.ClientSocket) error {
	return fmt.Errorf("not implemented")
}

// Not implemented common/data_type/database.Crud interface
//
// Returns an error
func (conf *Configuration) SelectAllByCondition(_ *remote.ClientSocket, _ key_value.KeyValue, _ interface{}) error {
	return fmt.Errorf("not implemented")
}

// Not implemented common/data_type/database.Crud interface
//
// Returns an error
func (conf *Configuration) Exist(_ *remote.ClientSocket) bool {
	return false
}

// Not implemented common/data_type/database.Crud interface
//
// Returns an error
func (conf *Configuration) Update(_ *remote.ClientSocket, _ uint8) error {
	return fmt.Errorf("not implemented")
}
