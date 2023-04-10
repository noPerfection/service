// The database package handles all the database operations.
// Note that for now it uses Mysql as a hardcoded data
//
// The database is creating a new service with the inproc reply controller.
// For any database operation interact with the service.
package db

import (
	"github.com/blocklords/sds/app/command"
	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"
)

const (
	READ_ROW command.CommandName = "read-row" // Get one row, if it doesn't exist, return error
	READ_ALL command.CommandName = "read"     // Read multiple line
	WRITE    command.CommandName = "write"    // insert or update
	EXIST    command.CommandName = "exist"    // Returns true or false if select query has some rows
	DELETE   command.CommandName = "delete"   // Delete some rows from database
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

// Run the database layer as a concurrent service
func Run(app_config *configuration.Config, parent log.Logger) {
	logger, err := parent.ChildWithTimestamp("database")
	if err != nil {
		parent.Fatal("logger.Child", "error", err)
	}

	// create a database connection
	// if security is enabled, then get the database credentials from vault
	// Set the database connection
	app_config.SetDefaults(DatabaseConfigurations)
	database_parameters, err := GetParameters(app_config)
	if err != nil {
		logger.Fatal("GetParameters", "error", err)
	}
	database_credentials := GetDefaultCredentials(app_config)
	// if secure is enabled
	// then get credentials from vault
	database, err := Open(logger, database_parameters, database_credentials)
	if err != nil {
		logger.Fatal("database error", "message", err)
	}
	// if app_config.Secure {
	// go vault_database.PeriodicallyRenewLeases(database.Reconnect)
	// }

	defer func() {
		_ = database.Close()
	}()

	db_service, err := service.Inprocess(service.DATABASE)
	if err != nil {
		logger.Fatal("service.Inproc", "service type", service.DATABASE, "error", err)
	}

	reply, err := controller.NewReply(db_service, logger)
	if err != nil {
		logger.Fatal("controller.NewReply", "url", db_service.Url(), "error", err)
	}

	command_handlers := command.EmptyHandlers().
		Add(READ_ROW, on_read_row).
		Add(DELETE, on_delete).
		Add(WRITE, on_write)

	reply.Run(command_handlers, database)
}

// Read the row only once
// func on_read_one_row(db *sql.DB, query string, parameters []interface{}, outputs []interface{}) ([]interface{}, error) {
func on_read_row(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	if len(parameters) == 0 {
		return message.Fail("the database connection wasn't passed to handler")
	}

	//parameters []interface{}, outputs []interface{}
	var query_parameters DatabaseQueryRequest
	err := request.Parameters.ToInterface(&query_parameters)
	if err != nil {
		return message.Fail("parameter validation:" + err.Error())
	}

	db, ok := parameters[0].(*Database)
	if !ok {
		return message.Fail("the parameter is not a database")
	}

	err = db.Connection.QueryRow(query_parameters.Query, query_parameters.Arguments...).Scan(query_parameters.Outputs...)
	if err != nil {
		return message.Fail("db.Connection.QueryRow: " + err.Error())
	}

	reply := ReadRowReply{
		Outputs: query_parameters.Outputs,
	}
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("command.Reply: " + err.Error())
	}

	return reply_message
}

// Execute the deletion
func on_delete(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	if len(parameters) == 0 {
		return message.Fail("the database connection wasn't passed to handler")
	}

	//parameters []interface{}, outputs []interface{}
	var query_parameters DatabaseQueryRequest
	err := request.Parameters.ToInterface(&query_parameters)
	if err != nil {
		return message.Fail("parameter validation:" + err.Error())
	}

	db, ok := parameters[0].(*Database)
	if !ok {
		return message.Fail("the parameter is not a database")
	}

	result, err := db.Connection.Exec(query_parameters.Query, query_parameters.Arguments...)
	if err != nil {
		return message.Fail("db.Connection.Exec: " + err.Error())
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return message.Fail("result.RowsAffected: " + err.Error())
	}

	if affected == 0 {
		return message.Fail("no rows were deleted")
	}
	reply := DeleteReply{}
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("command.Reply: " + err.Error())
	}

	return reply_message
}

// Execute the insert or update
func on_write(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	if len(parameters) == 0 {
		return message.Fail("the database connection wasn't passed to handler")
	}

	//parameters []interface{}, outputs []interface{}
	var query_parameters DatabaseQueryRequest
	err := request.Parameters.ToInterface(&query_parameters)
	if err != nil {
		return message.Fail("parameter validation:" + err.Error())
	}

	db, ok := parameters[0].(*Database)
	if !ok {
		return message.Fail("the parameter is not a database")
	}

	result, err := db.Connection.Exec(query_parameters.Query, query_parameters.Arguments...)
	if err != nil {
		return message.Fail("db.Connection.Exec: " + err.Error())
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return message.Fail("result.RowsAffected: " + err.Error())
	}

	if affected == 0 {
		return message.Fail("no rows were inserted or updated")
	}
	reply := WriteReply{}
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("command.Reply: " + err.Error())
	}

	return reply_message
}
