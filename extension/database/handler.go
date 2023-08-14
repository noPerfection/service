// Package database keeps the commands
package database

import (
	"github.com/ahmetson/common-lib/data_type"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/common-lib/message"
)

const (
	SelectRow string = "select-row" // Get one row, if it doesn't exist, return error
	SelectAll string = "select"     // Read multiple line
	Insert    string = "insert"     // insert new row
	Update    string = "update"     // update the existing row
	Exist     string = "exist"      // Returns true or false if select query has some rows
	Delete    string = "delete"     // Delete some rows from database
)

// QueryRequest has the sql and it's parameters on part with commands.
type QueryRequest struct {
	// Fields to manipulate,
	// for reading, it will have the SELECT clause fields
	//
	// for writing, it will have the Insert VALUES() clause fields
	Fields    []string      `json:"fields,omitempty"`
	Tables    []string      `json:"tables"`              // Tables that are used for query
	Where     string        `json:"where,omitempty"`     // WHERE part of the SQL query
	Arguments []interface{} `json:"arguments,omitempty"` // to pass in where clause
}

// SelectRowReply keeps the parameters of READ_ROW command reply by handler
type SelectRowReply struct {
	Outputs key_value.KeyValue `json:"outputs"` // all column parameters returned back to user
}

// SelectAllReply keeps the parameters of READ_ALL command reply by handler
type SelectAllReply struct {
	Rows []key_value.KeyValue `json:"rows"` // list of rows returned back to user
}

// InsertReply keeps the parameters of WRITE command reply by handler
type InsertReply struct {
	Id string `json:"id"`
}

// ExistReply keeps the parameters of Exist command reply by handler
type ExistReply struct {
	Exist bool `json:"exist"` // true or false
}

// DeleteReply keeps the parameters of Delete command reply by handler
type DeleteReply struct {
	Id string `json:"id"`
}

// UpdateReply keeps the parameters of Update command reply by handler
type UpdateReply struct {
	Id string `json:"id"`
}

// DeserializeBytes If no arguments were given, or no need to serialize, then return nil
func (requestParameters QueryRequest) DeserializeBytes() error {
	for i, rawArg := range requestParameters.Arguments {
		baseStr, ok := rawArg.(string)
		if !ok {
			continue
		}
		str := data_type.DecodeJsonPrefixed(baseStr)
		if len(str) > 0 {
			requestParameters.Arguments[i] = []byte(str)
			continue
		}
	}

	return nil
}

// Request returns a message.Request from the query request parameters
func (requestParameters QueryRequest) Request(commandName string) message.Request {
	parametersKv, _ := key_value.NewFromInterface(requestParameters)
	requestMessage := message.Request{
		Command:    commandName,
		Parameters: parametersKv,
	}

	return requestMessage
}
