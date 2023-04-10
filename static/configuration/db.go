package configuration

import (
	"fmt"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/common/topic"
	"github.com/blocklords/sds/db/handler"
)

// Inserts the configuration into the database
// It doesn't validates the configuration.
// Call conf.Validate() before calling this
func SetInDatabase(db *remote.ClientSocket, conf *Configuration) error {
	request := handler.DatabaseQueryRequest{
		Fields:    []string{"organization", "project", "network_id", "group_name", "smartcontract_name", "address"},
		Tables:    []string{"static_configuration"},
		Arguments: []interface{}{conf.Topic.Organization, conf.Topic.Project, conf.Topic.NetworkId, conf.Topic.Group, conf.Topic.Smartcontract, conf.Address},
	}
	var reply handler.InsertReply

	err := handler.INSERT.Request(db, request, &reply)
	if err != nil {
		return fmt.Errorf("handler.WRITE.Push: %w", err)
	}
	return nil
}

func GetAllFromDatabase(db *remote.ClientSocket) ([]*Configuration, error) {
	request := handler.DatabaseQueryRequest{
		Fields: []string{
			"organization as o",
			"project as p",
			"network_id as n",
			"group_name as g",
			"smartcontract_name as s",
			"address",
		},
		Tables: []string{"static_configuration"},
	}
	var reply handler.SelectAllReply

	err := handler.SELECT_ALL.Request(db, request, &reply)
	if err != nil {
		return nil, fmt.Errorf("handler.WRITE.Push: %w", err)
	}

	confs := make([]*Configuration, len(reply.Rows))

	// Loop through rows, using Scan to assign column data to struct fields.
	for i, raw := range reply.Rows {
		conf_topic, err := topic.ParseJSON(raw)
		if err != nil {
			return nil, fmt.Errorf("parsing topic parameters from database result failed: %w", err)
		}
		address, err := raw.GetString("address")
		if err != nil {
			return nil, fmt.Errorf("parsing address parameter from database result failed: %w", err)
		}
		conf, err := NewFromTopic(*conf_topic, address)
		if err != nil {
			return nil, fmt.Errorf("NewFromTopic: %w", err)
		}
		confs[i] = conf
	}
	return confs, err
}
