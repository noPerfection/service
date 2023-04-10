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
		Query:     `INSERT IGNORE INTO static_configuration (organization, project, network_id, group_name, smartcontract_name, address) VALUES (?, ?, ?, ?, ?, ?) `,
		Arguments: []interface{}{conf.Topic.Organization, conf.Topic.Project, conf.Topic.NetworkId, conf.Topic.Group, conf.Topic.Smartcontract, conf.Address},
	}
	var reply handler.WriteReply

	err := handler.WRITE.Request(db, request, &reply)
	if err != nil {
		return fmt.Errorf("handler.WRITE.Push: %w", err)
	}
	return nil
}

func GetAllFromDatabase(db *remote.ClientSocket) ([]*Configuration, error) {
	request := handler.DatabaseQueryRequest{
		Query:     "SELECT organization, project, network_id, group_name, smartcontract_name, address FROM static_configuration WHERE 1",
		Arguments: []interface{}{},
		Outputs:   []interface{}{"", "", "", "", "", ""},
	}
	var reply handler.ReadAllReply

	err := handler.WRITE.Request(db, request, &reply)
	if err != nil {
		return nil, fmt.Errorf("handler.WRITE.Push: %w", err)
	}

	confs := make([]*Configuration, len(reply.Rows))

	// Loop through rows, using Scan to assign column data to struct fields.
	for i, raw := range reply.Rows {
		conf := Configuration{
			Topic: topic.Topic{
				Organization:  raw.Outputs[0].(string),
				Project:       raw.Outputs[1].(string),
				NetworkId:     raw.Outputs[2].(string),
				Group:         raw.Outputs[3].(string),
				Smartcontract: raw.Outputs[4].(string),
			},
			Address: raw.Outputs[5].(string),
		}
		confs[i] = &conf
	}
	return confs, err
}
