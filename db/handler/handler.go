// Package handler lists the commands for database service.
package handler

import (
	"fmt"

	"github.com/blocklords/sds/app/communication/command"
	"github.com/blocklords/sds/common/data_type"
	"github.com/blocklords/sds/common/data_type/key_value"

	zmq "github.com/pebbe/zmq4"
)

const (
	NEW_CREDENTIALS command.CommandName = "new-credentials" // for pull controller, to receive credentials from vault
	SELECT_ROW      command.CommandName = "select-row"      // Get one row, if it doesn't exist, return error
	SELECT_ALL      command.CommandName = "select"          // Read multiple line
	INSERT          command.CommandName = "insert"          // insert new row
	UPDATE          command.CommandName = "update"          // update the existing row
	EXIST           command.CommandName = "exist"           // Returns true or false if select query has some rows
	DELETE          command.CommandName = "delete"          // Delete some rows from database
)

// DatabaseQueryRequest has the sql and it's parameters on part with commands.
type DatabaseQueryRequest struct {
	// Fields to manipulate,
	// for reading, it will have the SELECT clause fields
	//
	// for writing, it will have the INSERT VALUES() clause fields
	Fields    []string      `json:"fields,omitempty"`
	Tables    []string      `json:"tables"`              // Tables that are used for query
	Where     string        `json:"where,omitempty"`     // WHERE part of the SQL query
	Arguments []interface{} `json:"arguments,omitempty"` // to pass in where clause
}

// SelectRowReply keeps the parameters of READ_ROW command reply by controller
type SelectRowReply struct {
	Outputs key_value.KeyValue `json:"outputs"` // all column parameters returned back to user
}

// SelectAllReply keeps the parameters of READ_ALL command reply by controller
type SelectAllReply struct {
	Rows []key_value.KeyValue `json:"rows"` // list of rows returned back to user
}

// InsertReply keeps the parameters of WRITE command reply by controller
type InsertReply struct{}

// ExistReply keeps the parameters of EXIST command reply by controller
type ExistReply struct {
	Exist bool `json:"exist"` // true or false
}

// DeleteReply keeps the parameters of DELETE command reply by controller
type DeleteReply struct{}

// UpdateReply keeps the parameters of UPDATE command reply by controller
type UpdateReply struct{}

// PullerEndpoint returns the inproc pull controller to
// database.
//
// The pull controller receives the message from database
func PullerEndpoint() string {
	return "inproc://database_renew"
}

// PushSocket creates a client socket to the database puller.
//
// Used by the database credentials handler (for example: vault) to send to the database service
// new credentials.
func PushSocket() (*zmq.Socket, error) {
	sock, err := zmq.NewSocket(zmq.PUSH)
	if err != nil {
		return nil, fmt.Errorf("zmq error for new push socket: %w", err)
	}

	if err := sock.Connect(PullerEndpoint()); err != nil {
		return nil, fmt.Errorf("socket.Connect: %s: %w", PullerEndpoint(), err)
	}

	return sock, nil
}

// the bytes array are accepted as base64 string with "==" tail.
// deserialize it into the sequence of the bytes.
//
// If no arguments were given, or no need to serialize, then return nil
func (request DatabaseQueryRequest) DeserializeBytes() error {
	for i, raw_arg := range request.Arguments {
		base_str, ok := raw_arg.(string)
		if !ok {
			continue
		}
		str := data_type.DecodeJsonPrefixed(base_str)
		if len(str) > 0 {
			request.Arguments[i] = []byte(str)
			continue
		}
	}

	return nil
}

// BuildSelectQuery creates a SELECT SQL query
func (request DatabaseQueryRequest) BuildSelectQuery() (string, error) {
	if len(request.Fields) == 0 {
		return "", fmt.Errorf("missing Fields parameter")
	}
	if len(request.Tables) == 0 {
		return "", fmt.Errorf("missing Tables parameter")
	}
	if len(request.Where) > 0 && len(request.Arguments) == 0 {
		return "", fmt.Errorf("missing Arguments for Where clause")
	}

	str := `SELECT `

	last_field_index := len(request.Fields) - 1
	for i, field := range request.Fields {
		str += field
		if i < last_field_index {
			str += `, `
		}
	}

	str += ` FROM `
	last_table_index := len(request.Tables) - 1
	for i, table := range request.Tables {
		str += table
		if i < last_table_index {
			str += `, `
		}
	}

	str += ` WHERE `
	if len(request.Where) == 0 {
		return str + ` 1 `, nil
	} else {
		return str + request.Where, nil
	}
}

func (request DatabaseQueryRequest) BuildExistQuery() (string, error) {
	if len(request.Arguments) == 0 {
		return "", fmt.Errorf("missing Arguments parameter")
	}
	if len(request.Tables) == 0 {
		return "", fmt.Errorf("missing Tables parameter")
	}
	if len(request.Where) == 0 {
		return "", fmt.Errorf("missing Where parameter")
	}

	str := `SELECT 1 FROM `
	last_table_index := len(request.Tables) - 1
	for i, table := range request.Tables {
		str += table
		if i < last_table_index {
			str += `, `
		}
	}

	str += ` WHERE ` + request.Where
	return str, nil
}

// BuildSelectRowQuery creates a SELECT SQL query for fetching one row
func (request DatabaseQueryRequest) BuildSelectRowQuery() (string, error) {
	query, err := request.BuildSelectQuery()
	if err != nil {
		return "", fmt.Errorf("BuildSelectQuery: %w", err)
	}

	return query + " LIMIT 1 ", nil
}

// BuildUpdateQuery creates an UPDATE SQL query
func (request DatabaseQueryRequest) BuildUpdateQuery() (string, error) {
	if len(request.Fields) == 0 {
		return "", fmt.Errorf("missing Fields parameter")
	}
	if len(request.Tables) == 0 {
		return "", fmt.Errorf("missing Tables parameter")
	}
	if len(request.Arguments) == 0 {
		return "", fmt.Errorf("missing Arguments parameter")
	}
	if len(request.Where) == 0 {
		return "", fmt.Errorf("missing Where parameter, updating all rows is prohibited")
	}

	str := `UPDATE `
	// tables
	last_table_index := len(request.Tables) - 1
	for i, table := range request.Tables {
		str += table
		if i < last_table_index {
			str += `, `
		}
	}

	str += ` SET `
	// the fields
	last_field_index := len(request.Fields) - 1
	for i, field := range request.Fields {
		str += field + " = ?"
		if i < last_field_index {
			str += `, `
		}
	}
	str += " WHERE " + request.Where

	return str, nil
}

// BuildInsertRowQuery creates an INSERT INTO SQL query
func (request DatabaseQueryRequest) BuildInsertRowQuery() (string, error) {
	if len(request.Fields) == 0 {
		return "", fmt.Errorf("missing Fields parameter")
	}
	if len(request.Tables) == 0 {
		return "", fmt.Errorf("missing Tables parameter")
	}
	if len(request.Arguments) != len(request.Fields) {
		return "", fmt.Errorf("arguments to pass in insert clause mismatch")
	}

	str := `INSERT INTO `
	// tables
	last_table_index := len(request.Tables) - 1
	for i, table := range request.Tables {
		str += table
		if i < last_table_index {
			str += `, `
		}
	}

	str += ` (`
	// the fields
	last_field_index := len(request.Fields) - 1
	for i, field := range request.Fields {
		str += field
		if i < last_field_index {
			str += `, `
		}
	}

	str += `) VALUES ( `
	for i := range request.Fields {
		str += `?`
		if i < last_field_index {
			str += `, `
		}
	}
	str += `) `

	return str, nil
}

// BuildDeleteQuery creates DELETE FROM SQL query
func (request DatabaseQueryRequest) BuildDeleteQuery() (string, error) {
	if len(request.Fields) == 0 {
		return "", fmt.Errorf("missing Fields parameter")
	}
	if len(request.Tables) == 0 {
		return "", fmt.Errorf("missing Tables parameter")
	}

	str := `DELETE FROM `
	// tables
	last_table_index := len(request.Tables) - 1
	for i, table := range request.Tables {
		str += table
		if i < last_table_index {
			str += `, `
		}
	}

	if len(request.Where) == 0 {
		return str, nil
	}

	str += ` WHERE `
	// the fields
	last_field_index := len(request.Fields) - 1
	for i, field := range request.Fields {
		str += field
		if i < last_field_index {
			str += ` AND `
		}
	}

	return str, nil
}
