package configuration

import (
	"fmt"
	"strings"

	"github.com/blocklords/sds/common/topic"
	"github.com/blocklords/sds/db"
)

// Inserts the configuration into the database
func SetInDatabase(db *db.Database, conf *Configuration) error {
	_, err := db.Connection.Exec(`INSERT IGNORE INTO static_configuration (organization, project, network_id, group_name, smartcontract_name, address) VALUES (?, ?, ?, ?, ?, ?) `,
		conf.Organization, conf.Project, conf.NetworkId, conf.Group, conf.Name, conf.Address)
	if err != nil {
		fmt.Println("Failed to insert static configuration")
		return err
	}

	return nil

}

// Fills the static configuration parameters from database
func LoadDatabaseParts(db *db.Database, conf *Configuration) error {
	var address string

	err := db.Connection.QueryRow(`SELECT address FROM static_configuration WHERE 
	organization = ? AND project = ? AND network_id = ? AND group_name = ? AND 
	smartcontract_name = ? `, conf.Organization, conf.Project,
		conf.NetworkId, conf.Group, conf.Name).Scan(&address)
	if err != nil {
		fmt.Println("Loading static configuration parts returned db error: ", err.Error())
		return err
	}

	conf.SetAddress(address)

	return nil
}

// Whether the configuration exist in the database or not
func ExistInDatabase(db *db.Database, conf *Configuration) bool {
	var exists bool
	err := db.Connection.QueryRow(`SELECT IF(COUNT(address),'true','false') FROM static_configuration WHERE 
	organization = ? AND project = ? AND network_id = ? AND group_name = ? AND 
	smartcontract_name = ? `, conf.Organization, conf.Project,
		conf.NetworkId, conf.Group, conf.Name).Scan(&exists)
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

	l := t.Len(topic.ORGANIZATION_LEVEL)
	if l > 0 {
		query += ` AND static_configuration.organization IN (?` + strings.Repeat(",?", l-1) + `)`
		args = append(args, t.Organizations...)
	}

	l = t.Len(topic.PROJECT_LEVEL)
	if l > 0 {
		query += ` AND static_configuration.project IN (?` + strings.Repeat(",?", l-1) + `)`
		args = append(args, t.Projects...)
	}

	l = t.Len(topic.NETWORK_ID_LEVEL)
	if l > 0 {
		query += ` AND static_configuration.network_id IN (?` + strings.Repeat(",?", l-1) + `)`
		args = append(args, t.NetworkIds...)
	}

	l = t.Len(topic.GROUP_LEVEL)
	if l > 0 {
		query += ` AND static_configuration.group_name IN (?` + strings.Repeat(",?", l-1) + `)`
		args = append(args, t.Groups...)
	}

	l = t.Len(topic.SMARTCONTRACT_LEVEL)
	if l > 0 {
		query += ` AND static_configuration.smartcontract_name IN (?` + strings.Repeat(",?", l-1) + `)`
		args = append(args, t.Smartcontracts...)
	}

	return query, args
}
