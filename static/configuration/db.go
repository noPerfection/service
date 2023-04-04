package configuration

import (
	"fmt"
	"strings"

	"github.com/blocklords/sds/common/topic"
	"github.com/blocklords/sds/db"
)

// Inserts the configuration into the database
// It doesn't validates the configuration.
// Call conf.Validate() before calling this
func SetInDatabase(db *db.Database, conf *Configuration) error {
	result, err := db.Connection.Exec(`INSERT IGNORE INTO static_configuration (organization, project, network_id, group_name, smartcontract_name, address) VALUES (?, ?, ?, ?, ?, ?) `,
		conf.Topic.Organization, conf.Topic.Project, conf.Topic.NetworkId, conf.Topic.Group, conf.Topic.Smartcontract, conf.Address)
	if err != nil {
		fmt.Println("Failed to insert static configuration")
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking insert result: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("expected to have 1 affected rows. Got %d", affected)
	}

	return nil

}

// Adds the address to the configuration
func LoadDatabaseParts(db *db.Database, conf *Configuration) error {
	var address string

	err := db.Connection.QueryRow(`SELECT address FROM static_configuration WHERE 
	organization = ? AND project = ? AND network_id = ? AND group_name = ? AND 
	smartcontract_name = ? `, conf.Topic.Organization, conf.Topic.Project,
		conf.Topic.NetworkId, conf.Topic.Group, conf.Topic.Smartcontract).Scan(&address)
	if err != nil {
		fmt.Println("Loading static configuration parts returned db error: ", err.Error())
		return err
	}

	conf.Address = address

	return nil
}

// Whether the configuration exist in the database or not
func ExistInDatabase(db *db.Database, topic *topic.Topic) bool {
	var exists bool
	err := db.Connection.QueryRow(`SELECT IF(COUNT(address),'true','false') FROM static_configuration WHERE 
	organization = ? AND project = ? AND network_id = ? AND group_name = ? AND 
	smartcontract_name = ? `, topic.Organization, topic.Project,
		topic.NetworkId, topic.Group, topic.Smartcontract).Scan(&exists)
	if err != nil {
		fmt.Println("Static Configuration exists returned db error: ", err.Error())
		return false
	}

	return exists
}

// Creates a database query that will be used to query smartcontracts
func QueryFilterSmartcontract(t *topic.TopicFilter) (string, []string) {
	query := ""
	args := make([]string, 0)

	l := len(t.Organizations)
	if l > 0 {
		query += ` AND static_configuration.organization IN (?` + strings.Repeat(",?", l-1) + `)`
		args = append(args, t.Organizations...)
	}

	l = len(t.Projects)
	if l > 0 {
		query += ` AND static_configuration.project IN (?` + strings.Repeat(",?", l-1) + `)`
		args = append(args, t.Projects...)
	}

	l = len(t.NetworkIds)
	if l > 0 {
		query += ` AND static_configuration.network_id IN (?` + strings.Repeat(",?", l-1) + `)`
		args = append(args, t.NetworkIds...)
	}

	l = len(t.Groups)
	if len(t.Groups) > 0 {
		query += ` AND static_configuration.group_name IN (?` + strings.Repeat(",?", l-1) + `)`
		args = append(args, t.Groups...)
	}

	l = len(t.Smartcontracts)
	if len(t.Smartcontracts) > 0 {
		query += ` AND static_configuration.smartcontract_name IN (?` + strings.Repeat(",?", l-1) + `)`
		args = append(args, t.Smartcontracts...)
	}

	return query, args
}
