// Package handler lists the commands for database service.
package handler

import (
	"fmt"

	"github.com/blocklords/sds/app/command"

	zmq "github.com/pebbe/zmq4"
)

const (
	NEW_CREDENTIALS command.CommandName = "new-credentials" // for pull controller, to receive credentials from vault
	READ_ROW        command.CommandName = "read-row"        // Get one row, if it doesn't exist, return error
	READ_ALL        command.CommandName = "read"            // Read multiple line
	WRITE           command.CommandName = "write"           // insert or update
	EXIST           command.CommandName = "exist"           // Returns true or false if select query has some rows
	DELETE          command.CommandName = "delete"          // Delete some rows from database
)

// DatabaseQueryRequest has the sql and it's parameters on part with commands.
type DatabaseQueryRequest struct {
	Query     string        `json:"query"`             // SQL query to apply
	Arguments []interface{} `json:"arguments"`         // Parameters to insert into SQL query
	Outputs   []interface{} `json:"outputs,omitempty"` // For reading it will keep what kind of data parameters to get
}

// ReadRowReply keeps the parameters of READ_ROW command reply by controller
type ReadRowReply struct {
	Outputs []interface{} `json:"outputs"` // all column parameters returned back to user
}

// ReadAllReply keeps the parameters of READ_ALL command reply by controller
type ReadAllReply struct {
	Rows []ReadRowReply `json:"rows"` // list of rows returned back to user
}

// WriteReply keeps the parameters of WRITE command reply by controller
type WriteReply struct{}

// ExistReply keeps the parameters of EXIST command reply by controller
type ExistReply struct {
	Exist bool `json:"exist"` // true or false
}

// DeleteReply keeps the parameters of DELETE command reply by controller
type DeleteReply struct{}

// PullerEndpoint returns the inproc pull controller to
// database.
//
// The pull controller receives the message from database
func PullerEndpoint() string {
	return "inproc://database_renew"
}

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
