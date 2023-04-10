// The database package handles all the database operations.
// Note that for now it uses Mysql as a hardcoded data
//
// The database is creating a new service with the inproc reply controller.
// For any database operation interact with the service.
package db

import (
	"context"
	"sync"

	"github.com/blocklords/sds/app/command"
	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/db/handler"
)

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

	var database *Database
	if app_config.Secure {
		database = &Database{
			Connection:      nil,
			connectionMutex: sync.Mutex{},
			parameters:      *database_parameters,
			logger:          logger,
		}
		// vault will push the credentials here
		database.run_puller()
	} else {
		database_credentials := GetDefaultCredentials(app_config)
		database, err = Open(logger, database_parameters, database_credentials)
		if err != nil {
			logger.Fatal("database error", "message", err)
		}
	}

	go database.run_controller()
}

// run_puller creates a pull controller that get's the
// new database credentials to reconnect.
func (database *Database) run_puller() {
	db_service, err := service.InprocessFromUrl(handler.PullerEndpoint())
	if err != nil {
		database.logger.Fatal("service.Inproc", "service type", service.DATABASE, "error", err)
	}

	pull, err := controller.NewPull(db_service, database.logger)
	if err != nil {
		database.logger.Fatal("controller.NewReply", "url", db_service.Url(), "error", err)
	}

	command_handlers := command.EmptyHandlers().
		Add(handler.NEW_CREDENTIALS, on_new_credentials)

	pull.Run(command_handlers, database)
}

// run_controller creates a reply controller for other services
// to interact with the database
func (database *Database) run_controller() {
	db_service, err := service.Inprocess(service.DATABASE)
	if err != nil {
		database.logger.Fatal("service.Inproc", "service type", service.DATABASE, "error", err)
	}

	reply, err := controller.NewReply(db_service, database.logger)
	if err != nil {
		database.logger.Fatal("controller.NewReply", "url", db_service.Url(), "error", err)
	}

	command_handlers := command.EmptyHandlers().
		Add(handler.READ_ROW, on_read_row).
		Add(handler.DELETE, on_delete).
		Add(handler.WRITE, on_write)

	reply.Run(command_handlers, database)
}

// puller received new credentials
func on_new_credentials(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	if len(parameters) == 0 {
		return message.Fail("the database connection wasn't passed to handler")
	}

	db, ok := parameters[0].(*Database)
	if !ok {
		return message.Fail("the parameter is not a database")
	}

	var credentials DatabaseCredentials
	err := request.Parameters.ToInterface(&credentials)
	if err != nil {
		return message.Fail("the received database credentials are invalid")
	}

	if db.Connection == nil {
		return message.Fail("database.Connection is nil, please open the connection first")
	}

	ctx := context.TODO()

	// establish the first connection
	if err := db.Reconnect(ctx, credentials); err != nil {
		return message.Fail("database.reconnect:" + err.Error())
	}

	return message.Reply{
		Status:     message.OK,
		Message:    "",
		Parameters: key_value.Empty(),
	}
}

// Read the row only once
// func on_read_one_row(db *sql.DB, query string, parameters []interface{}, outputs []interface{}) ([]interface{}, error) {
func on_read_row(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	if len(parameters) == 0 {
		return message.Fail("the database connection wasn't passed to handler")
	}

	db, ok := parameters[0].(*Database)
	if !ok {
		return message.Fail("the parameter is not a database")
	}
	if db.Connection == nil {
		return message.Fail("database.Connection is nil, please open the connection first")
	}

	//parameters []interface{}, outputs []interface{}
	var query_parameters handler.DatabaseQueryRequest
	err := request.Parameters.ToInterface(&query_parameters)
	if err != nil {
		return message.Fail("parameter validation:" + err.Error())
	}

	err = db.Connection.QueryRow(query_parameters.Query, query_parameters.Arguments...).Scan(query_parameters.Outputs...)
	if err != nil {
		return message.Fail("db.Connection.QueryRow: " + err.Error())
	}

	reply := handler.ReadRowReply{
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

	db, ok := parameters[0].(*Database)
	if !ok {
		return message.Fail("the parameter is not a database")
	}
	if db.Connection == nil {
		return message.Fail("database.Connection is nil, please open the connection first")
	}

	//parameters []interface{}, outputs []interface{}
	var query_parameters handler.DatabaseQueryRequest
	err := request.Parameters.ToInterface(&query_parameters)
	if err != nil {
		return message.Fail("parameter validation:" + err.Error())
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
	reply := handler.DeleteReply{}
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

	db, ok := parameters[0].(*Database)
	if !ok {
		return message.Fail("the parameter is not a database")
	}
	if db.Connection == nil {
		return message.Fail("database.Connection is nil, please open the connection first")
	}

	//parameters []interface{}, outputs []interface{}
	var query_parameters handler.DatabaseQueryRequest
	err := request.Parameters.ToInterface(&query_parameters)
	if err != nil {
		return message.Fail("parameter validation:" + err.Error())
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
	reply := handler.WriteReply{}
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("command.Reply: " + err.Error())
	}

	return reply_message
}
