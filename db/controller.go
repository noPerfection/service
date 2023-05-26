// The database package handles all the database operations.
// Note that for now it uses Mysql as a hardcoded data
//
// The database is creating a new service with the inproc reply controller.
// For any database operation interact with the service.
package db

import (
	"database/sql"
	"sync"

	"github.com/blocklords/sds/app/communication/command"
	"github.com/blocklords/sds/app/communication/message"
	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/common/data_type/database"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/db/handler"
)

// Run the database servce
func Run(app_config *configuration.Config) {
	logger, err := log.New("database", log.WITH_TIMESTAMP)
	if err != nil {
		log.Fatal("log.Child", "error", err)
	}

	logger.Info("Starting with setting of the default parameters")

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
		logger.Info("Security enabled, therefore start pull controller that waits credentials from vault service")
		database = &Database{
			Connection:      nil,
			connectionMutex: sync.Mutex{},
			parameters:      *database_parameters,
			logger:          logger,
		}
		// vault will push the credentials here
		database.run_puller()
	} else {
		logger.Info("Database is connected in an unsafe way. Connecting with default credentials")

		database, err = connect_with_default(app_config, logger, database_parameters)
		if err != nil {
			logger.Fatal("database error", "message", err)
		}
		logger.Info("Database connected successfully!")
	}

	logger.Info("Run database controller")
	go database.run_controller()
}

// run_puller creates a pull controller that get's the
// new database credentials to reconnect.
func (database *Database) run_puller() {
	database.logger.Info("Creating puller service to get credentials from vault service", "url", handler.PullerEndpoint())

	puller_service, err := service.InprocessFromUrl(handler.PullerEndpoint())
	if err != nil {
		database.logger.Fatal("service.Inproc", "service type", service.DATABASE, "error", err)
	}

	pull, err := controller.NewPull(puller_service, database.logger)
	if err != nil {
		database.logger.Fatal("controller.NewPull", "url", puller_service.Url(), "error", err)
	}

	command_handlers := command.EmptyHandlers().
		Add(handler.NEW_CREDENTIALS, on_new_credentials)

	database.logger.Info("Running pull controller")
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
		Add(handler.EXIST, on_exist).
		Add(handler.SELECT_ROW, on_select_row).
		Add(handler.SELECT_ALL, on_select_all).
		Add(handler.DELETE, on_delete).
		Add(handler.INSERT, on_insert).
		Add(handler.UPDATE, on_update)

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

	// establish the first connection
	if err := db.Reconnect(credentials); err != nil {
		return message.Fail("database.reconnect:" + err.Error())
	}

	return message.Reply{
		Status:     message.OK,
		Message:    "",
		Parameters: key_value.Empty(),
	}
}

// selects all rows from the database
//
// intended to be used once during the app launch for caching.
//
// Minimize the database queries by using this
func on_select_all(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
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

	query, err := query_parameters.BuildSelectQuery()
	if err != nil {
		return message.Fail("query_parameter.BuildSelectQuery: " + err.Error())
	}

	var rows *sql.Rows
	if len(query_parameters.Where) > 0 {
		rows, err = db.Connection.Query(query, query_parameters.Arguments...)
	} else {
		rows, err = db.Connection.Query(query)
	}
	if err != nil {
		return message.Fail("db.Connection.Query: " + err.Error())
	}
	field_types, err := rows.ColumnTypes()
	if err != nil {
		return message.Fail("rows.ColumnTypes: " + err.Error())
	}

	reply_objects := make([]key_value.KeyValue, 0)

	for rows.Next() {
		scans := make([]interface{}, len(field_types))
		row := key_value.Empty()

		for i := range scans {
			scans[i] = &scans[i]
		}
		rows.Scan(scans...)
		for i, v := range scans {
			err := database.SetValue(row, field_types[i], v)
			if err != nil {
				return message.Fail("failed to set value for field " + field_types[i].Name() + " of " + field_types[i].DatabaseTypeName() + " type: " + err.Error())
			}
		}

		reply_objects = append(reply_objects, key_value.New(row))
	}

	reply := handler.SelectAllReply{
		Rows: reply_objects,
	}
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("command.Reply: " + err.Error())
	}

	return reply_message
}

// checks whether there are any rows that matches to the query
func on_exist(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
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

	query, err := query_parameters.BuildExistQuery()
	if err != nil {
		return message.Fail("query_parameter.BuildExistQuery: " + err.Error())
	}

	rows, err := db.Connection.Query(query, query_parameters.Arguments...)
	if err != nil {
		return message.Fail("db.Connection.Query: " + err.Error())
	}
	defer rows.Close()

	reply := handler.ExistReply{}

	if rows.Next() {
		reply.Exist = true
	} else {
		reply.Exist = false
	}

	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("command.Reply: " + err.Error())
	}

	return reply_message
}

// Read the row only once
// func on_read_one_row(db *sql.DB, query string, parameters []interface{}, outputs []interface{}) ([]interface{}, error) {
func on_select_row(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
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

	query, err := query_parameters.BuildSelectRowQuery()
	if err != nil {
		return message.Fail("query_parameter.BuildSelectRowQuery: " + err.Error())
	}

	rows, err := db.Connection.Query(query, query_parameters.Arguments...)
	if err != nil {
		return message.Fail("db.Connection.Query: " + err.Error())
	}
	defer rows.Close()

	field_types, err := rows.ColumnTypes()
	if err != nil {
		return message.Fail("rows.ColumnTypes: " + err.Error())
	}

	row := key_value.Empty()
	no_result := true

	for rows.Next() {
		no_result = false
		scans := make([]interface{}, len(field_types))

		for i := range scans {
			scans[i] = &scans[i]
		}
		rows.Scan(scans...)
		for i, v := range scans {
			err := database.SetValue(row, field_types[i], v)
			if err != nil {
				return message.Fail("failed to set value for field " + field_types[i].Name() + " of " + field_types[i].DatabaseTypeName() + " type: " + err.Error())
			}
		}
	}

	if no_result {
		return message.Fail("not found")
	}

	reply := handler.SelectRowReply{
		Outputs: key_value.New(row),
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

	query, err := query_parameters.BuildDeleteQuery()
	if err != nil {
		return message.Fail("query_parameter.BuildDeleteQuery: " + err.Error())
	}

	result, err := db.Connection.Exec(query, query_parameters.Arguments...)
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

// Execute the insert
func on_insert(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
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

	err = query_parameters.DeserializeBytes()
	if err != nil {
		return message.Fail("serialization failed: %w" + err.Error())
	}

	query, err := query_parameters.BuildInsertRowQuery()
	if err != nil {
		return message.Fail("query_parameter.BuildInsertRowQuery: " + err.Error())
	}

	result, err := db.Connection.Exec(query, query_parameters.Arguments...)
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
	reply := handler.InsertReply{}
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("command.Reply: " + err.Error())
	}

	return reply_message
}

// Execute the row update
func on_update(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
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

	err = query_parameters.DeserializeBytes()
	if err != nil {
		return message.Fail("serialization failed: %w" + err.Error())
	}

	query, err := query_parameters.BuildUpdateQuery()
	if err != nil {
		return message.Fail("query_parameter.BuildUpdateQuery: " + err.Error())
	}

	result, err := db.Connection.Exec(query, query_parameters.Arguments...)
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
	reply := handler.UpdateReply{}
	reply_message, err := command.Reply(&reply)
	if err != nil {
		return message.Fail("command.Reply: " + err.Error())
	}

	return reply_message
}
