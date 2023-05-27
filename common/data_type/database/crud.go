package database

import (
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/service/remote"
)

// Struct interface adds the database CRUD to the data struct.
type Crud interface {
	// Update the parameters by int flag. It calls UPDATE command
	Update(*remote.ClientSocket, uint8) error
	// Exist in the database or not. It calls EXIST command
	Exist(*remote.ClientSocket) bool

	// Insert into the database. It calls INSERT command
	Insert(*remote.ClientSocket) error
	// Load the database from database. It calls SELECT_ROW command
	Select(*remote.ClientSocket) error

	// It calls SELECT_ALL without WHERE clause of query.
	//
	// Result is then put to the second argument
	SelectAll(*remote.ClientSocket, interface{}) error

	// AllByCondition returns structs from database to the second argument.
	// The sql query should match to the condition.
	//
	// It calls SELECT_ALL with WHERE clause
	SelectAllByCondition(*remote.ClientSocket, key_value.KeyValue, interface{}) error // uses SELECT_ROW
}
