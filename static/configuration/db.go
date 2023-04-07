package configuration

import (
	"fmt"

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

func GetAllFromDatabase(db *db.Database) ([]*Configuration, error) {
	rows, err := db.Connection.Query("SELECT organization, project, network_id, group_name, smartcontract_name, address FROM static_configuration WHERE 1")
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}

	defer rows.Close()

	configurations := make([]*Configuration, 0)

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var s = Configuration{
			Topic:   topic.Topic{},
			Address: "",
		}

		if err := rows.Scan(&s.Topic.Organization, &s.Topic.Project, &s.Topic.NetworkId, &s.Topic.Group, &s.Topic.Smartcontract, &s.Address); err != nil {
			return nil, fmt.Errorf("failed to scan database result: %w", err)
		}

		configurations = append(configurations, &s)
	}
	return configurations, err
}
