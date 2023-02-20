package static

import (
	"github.com/blocklords/gosds/security/vault"
	"github.com/blocklords/gosds/static/handler"

	"github.com/blocklords/gosds/app/configuration"
	"github.com/blocklords/gosds/app/service"

	"github.com/blocklords/gosds/app/account"

	"github.com/blocklords/gosds/app/controller"
	"github.com/blocklords/gosds/db"
)

func Run(app_config *configuration.Config, db_connection *db.Database, v *vault.Vault) {
	var commands = controller.CommandHandlers{
		"abi_get":                      handler.AbiGet,
		"abi_get_by_smartcontract_key": handler.AbiGetBySmartcontractKey,
		"abi_register":                 handler.AbiRegister,

		"smartcontract_get":        handler.SmartcontractGet,
		"smartcontract_register":   handler.SmartcontractRegister,
		"smartcontract_filter":     handler.SmartcontractFilter,
		"smartcontract_key_filter": handler.SmartcontractKeyFilter,

		"configuration_get":      handler.ConfigurationGet,
		"configuration_register": handler.ConfigurationRegister,

		"network_id_get_all": handler.NetworkGetIds,
		"network_get_all":    handler.NetworkGetAll,
		"network_get":        handler.NetworkGet,
	}

	// Getting the services which has access to the SDS Static
	static_env, err := service.New(service.STATIC, service.THIS)
	if err != nil {
		panic(err)
	}

	// The list of whitelisted SDS Services that can access to SDS Static:
	//
	//  * Developer Gateway
	//  * Gateway
	//  * Categorizer
	developer_gateway_env, err := service.New(service.DEVELOPER_GATEWAY, service.REMOTE)
	if err != nil {
		panic(static_env)
	}
	developer_gateway := account.NewService(developer_gateway_env)

	gateway_env, err := service.New(service.GATEWAY, service.REMOTE)
	if err != nil {
		panic(gateway_env)
	}
	gateway := account.NewService(gateway_env)

	categorizer_env, err := service.New(service.CATEGORIZER, service.REMOTE)
	if err != nil {
		panic(categorizer_env)
	}
	categorizer := account.NewService(categorizer_env)

	bundle_env, err := service.New(service.BUNDLE, service.REMOTE)
	if err != nil {
		panic(err)
	}
	bundle := account.NewService(bundle_env)

	log_env, err := service.New(service.LOG, service.REMOTE)
	if err != nil {
		panic(err)
	}
	log := account.NewService(log_env)

	reader_env, err := service.New(service.READER, service.REMOTE)
	if err != nil {
		panic(err)
	}
	reader := account.NewService(reader_env)

	writer_env, err := service.New(service.WRITER, service.REMOTE)
	if err != nil {
		panic(err)
	}
	writer := account.NewService(writer_env)

	// SDS Spaghetti fetches the network parameters from SDS Static
	spaghetti_env, err := service.New(service.SPAGHETTI, service.REMOTE)
	if err != nil {
		panic(err)
	}
	spaghetti := account.NewService(spaghetti_env)

	accounts := account.NewAccounts(developer_gateway, gateway, categorizer)
	accounts = accounts.Add(bundle, log, reader, writer, spaghetti)

	err = controller.ReplyController(db_connection, commands, static_env, accounts)
	if err != nil {
		panic(err)
	}
}
