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

	// we whitelist before we initiate the reply controller
	if !app_config.Plain {
		whitelisted_services, err := get_whitelisted_services()
		if err != nil {
			panic(err)
		}
		accounts := account.NewServices(whitelisted_services)
		controller.AddWhitelistedAccounts(static_env, accounts)
	}

	reply, err := controller.NewReply(static_env)
	if err != nil {
		panic(err)
	}

	if !app_config.Plain {
		err := reply.SetControllerPrivateKey()
		if err != nil {
			panic(err)
		}
	}

	err = controller.ReplyController(db_connection, commands, static_env)
	if err != nil {
		panic(err)
	}
}
