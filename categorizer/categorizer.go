package categorizer

import (
	debug_log "log"

	"github.com/blocklords/gosds/categorizer/handler"
	"github.com/blocklords/gosds/categorizer/imx"
	"github.com/blocklords/gosds/categorizer/smartcontract"
	"github.com/blocklords/gosds/categorizer/worker"
	"github.com/blocklords/gosds/common/data_type/key_value"
	"github.com/blocklords/gosds/static/network"

	"github.com/blocklords/gosds/app/account"
	"github.com/blocklords/gosds/app/argument"
	"github.com/blocklords/gosds/app/configuration"
	"github.com/blocklords/gosds/security/vault"

	"github.com/blocklords/gosds/app/broadcast"
	"github.com/blocklords/gosds/app/remote"

	"github.com/blocklords/gosds/app/remote/message"

	"github.com/blocklords/gosds/app/service"

	"github.com/blocklords/gosds/db"

	"github.com/blocklords/gosds/app/controller"
)

var broadcast_channel chan message.Broadcast
var spaghetti_in chan worker.RequestSpaghettiBlockRange
var spaghetti_out chan worker.ReplySpaghettiBlockRange
var spaghetti_socket *remote.Socket

var log_parse_in chan worker.RequestLogParse = nil
var log_parse_out chan worker.ReplyLogParse = nil

var static_socket *remote.Socket

var no_event bool = false

var imx_manager *imx.Manager = nil
var evm_managers map[string]*worker.Manager

// Manages the EVM based smartcontracts on a certain blockchain
func run_evm_manager(db_con *db.Database, network *network.Network) {
	// smartcontract.GetAll() is the first database connection.
	// therefore it checks database liveness.
	smartcontracts, err := smartcontract.GetAllByNetworkId(db_con, network.Id)
	if err != nil {
		panic(`error to fetch all categorized smartcontracts. received database error: ` + err.Error() + ` for network id ` + network.Id)
	}

	workers, err := worker.WorkersFromSmartcontracts(
		db_con,
		static_socket,
		smartcontracts,
		no_event,
		broadcast_channel,
		spaghetti_in,
		spaghetti_out,
		log_parse_in,
		log_parse_out,
	)
	if err != nil {
		panic("failed to create list of workers for network id " + network.Id)
	}

	manager := worker.NewManager(network.Id, spaghetti_socket, spaghetti_in, spaghetti_out)
	manager.In <- workers

	evm_managers[network.Id] = manager
}

// Manages the ImmutableX blockchain smartcontracts
func run_imx_manager(db_con *db.Database, network *network.Network) {
	// smartcontract.GetAll() is the first database connection.
	// therefore it checks database liveness.
	smartcontracts, err := smartcontract.GetAllByNetworkId(db_con, network.Id)
	if err != nil {
		panic(`error to fetch all categorized smartcontracts. received database error: ` + err.Error() + ` for network id ` + network.Id)
	}

	for _, sm := range smartcontracts {
		imx_manager.AddSmartcontract()

		go worker.ImxRun(db_con, sm, imx_manager, broadcast_channel)
	}
}

////////////////////////////////////////////////////////////////////
//
// Command handlers
//
////////////////////////////////////////////////////////////////////

// Saves the smartcontract in the database.
// if SDS Categorizer was running with broadcast enabled,
// then start a worker.
func smartcontract_set(db_con *db.Database, request message.Request) message.Reply {
	kv, err := request.Parameters.GetKeyValue("smartcontract")
	if err != nil {
		return message.Fail("missing 'smartcontract' parameter")
	}

	sm, err := smartcontract.New(kv)
	if err != nil {
		return message.Fail(err.Error())
	}

	if smartcontract.Exists(db_con, sm.NetworkId, sm.Address) {
		return message.Fail("the smartcontract already in SDS Categorizer")
	}

	saveErr := smartcontract.Save(db_con, sm)
	if saveErr != nil {
		return message.Fail(saveErr.Error())
	}

	if broadcast_enabled, _ := argument.Exist(argument.BROADCAST); broadcast_enabled {
		if sm.NetworkId == imx.NETWORK_ID {
			if imx_manager == nil {
				return message.Fail("unsupported network_id")
			}
			imx_manager.AddSmartcontract()
			go worker.ImxRun(db_con, sm, imx_manager, broadcast_channel)
		} else {
			manager, ok := evm_managers[sm.NetworkId]
			if !ok {
				return message.Fail("unsupported network_id")
			}

			workers, err := worker.WorkersFromSmartcontracts(
				db_con,
				static_socket,
				[]*smartcontract.Smartcontract{sm},
				no_event,
				broadcast_channel,
				spaghetti_in,
				spaghetti_out,
				log_parse_in,
				log_parse_out,
			)
			if err != nil {
				return message.Fail("failed to create list of workers for network id " + sm.NetworkId)
			}

			manager.In <- workers
		}
	}

	reply := message.Reply{
		Status:     "OK",
		Message:    "",
		Parameters: key_value.Empty().Set("smartcontract", sm),
	}

	return reply
}

// Smartcontract data are parsed and stored in the database
func Run(app_config *configuration.Config, db_con *db.Database, v *vault.Vault) {
	greeting := `SDS Categorizer preparing...
Supported command line arguments:
    --broadcast                 runs the broadcaster about categorized smartcontracts
    --reply                     runs the request-reply server
    --plain                     runs the servers without security
    --network-id=<network id>   runs the smartcontract workers for this network id only
    --no-event                  categorization will not parse the event logs
    --security-debug            prints the security logs`
	println(greeting + "\n\n")

	arguments, err := argument.GetArguments()
	if err != nil {
		panic(err)
	}
	// check for missing environment variable otherwise panic exit.
	if _, err := service.New(service.SPAGHETTI, service.SUBSCRIBE, service.REMOTE); err != nil {
		panic(err)
	}

	categorizer_env, err := service.New(service.CATEGORIZER, service.BROADCAST, service.THIS)
	if err != nil {
		panic(err)
	}

	spaghetti_env, err := service.New(service.SPAGHETTI, service.REMOTE)
	if err != nil {
		panic(err)
	}

	static_env, err := service.New(service.STATIC, service.REMOTE)
	if err != nil {
		panic(err)
	}

	log_env, err := service.New(service.LOG, service.REMOTE)
	if err != nil {
		panic(err)
	}

	developer_gateway_env, err := service.New(service.DEVELOPER_GATEWAY, service.REMOTE, service.SUBSCRIBE)
	if err != nil {
		panic(err)
	}

	publisher_env, err := service.New(service.PUBLISHER, service.REMOTE, service.SUBSCRIBE)
	if err != nil {
		panic(err)
	}

	gateway_env, err := service.New(service.GATEWAY, service.REMOTE)
	if err != nil {
		panic(err)
	}

	bundle_env, err := service.New(service.BUNDLE, service.REMOTE)
	if err != nil {
		panic(err)
	}

	reader_env, err := service.New(service.READER, service.REMOTE)
	if err != nil {
		panic(err)
	}

	writer_env, err := service.New(service.WRITER, service.REMOTE)
	if err != nil {
		panic(err)
	}

	if !app_config.Broadcast && !app_config.Reply {
		debug_log.Fatalf("'%s' missing --reply and/or --broadcast. Please pass it as an argument", categorizer_env.ServiceName())

	}

	spaghetti_socket = remote.TcpRequestSocketOrPanic(spaghetti_env, categorizer_env)
	static_socket = remote.TcpRequestSocketOrPanic(static_env, categorizer_env)

	no_event = argument.Has(arguments, argument.NO_EVENT)

	var networks network.Networks = make(network.Networks, 0)
	if argument.Has(arguments, argument.NETWORK_ID) {
		network_id, err := argument.ExtractValue(arguments, argument.NETWORK_ID)
		if err != nil {
			panic(err)
		}

		one_network, err := network.GetRemoteNetwork(static_socket, network_id, network.ALL)
		if err != nil {
			panic(err)
		}

		networks = append(networks, one_network)
	} else {
		networks, err = network.GetRemoteNetworks(static_socket, network.ALL)
		if err != nil {
			panic(err)
		}
	}

	if networks.Exist(imx.NETWORK_ID) {
		if err := imx.ValidateEnv(app_config); err != nil {
			panic(err)
		} else {
			imx_manager = imx.NewManager(app_config)
		}
	}

	if app_config.Broadcast {
		subscribers_env := []*service.Service{developer_gateway_env, publisher_env}

		broadcast_channel = make(chan message.Broadcast)
		spaghetti_in = make(chan worker.RequestSpaghettiBlockRange)
		spaghetti_out = make(chan worker.ReplySpaghettiBlockRange)

		if !no_event {
			log_parse_in = make(chan worker.RequestLogParse)
			log_parse_out = make(chan worker.ReplyLogParse)
			go worker.LogParse(log_parse_in, log_parse_out)
		}

		go worker.SpaghettiBlockRange(spaghetti_in, spaghetti_out)

		for _, network := range networks {
			if network.Id == imx.NETWORK_ID {
				run_imx_manager(db_con, network)
			} else {
				run_evm_manager(db_con, network)
			}
		}

		if app_config.Reply {
			go broadcast.Run(broadcast_channel, categorizer_env, subscribers_env)
		} else {
			broadcast.Run(broadcast_channel, categorizer_env, subscribers_env)
		}
	}

	if app_config.Reply {
		var commands = controller.CommandHandlers{
			"smartcontract_get_all": handler.GetSmartcontracts,
			"smartcontract_get":     handler.GetSmartcontract,

			"log_get_all": handler.GetLogs,

			"transaction_get_all": handler.GetTransactions,
			"transaction_amount":  handler.GetTransactionAmount,

			"snapshot_get": handler.GetSnapshot,

			"smartcontract_set": smartcontract_set,
		}

		// Allowed services to connect to SDS Categorizer
		accounts := account.NewAccounts(account.NewService(developer_gateway_env))
		accounts = accounts.Add(account.NewService(bundle_env), account.NewService(log_env))
		accounts = accounts.Add(account.NewService(reader_env), account.NewService(writer_env))
		accounts = accounts.Add(account.NewService(publisher_env), account.NewService(gateway_env))

		err := controller.ReplyController(db_con, commands, categorizer_env, accounts)
		if err != nil {
			panic(err)
		}
	}
}
