package categorizer

import (
	"errors"
	"fmt"

	"github.com/blocklords/gosds/categorizer/abi"
	"github.com/blocklords/gosds/categorizer/handler"
	"github.com/blocklords/gosds/categorizer/imx"
	"github.com/blocklords/gosds/categorizer/smartcontract"
	"github.com/blocklords/gosds/categorizer/worker"
	evm_worker "github.com/blocklords/gosds/categorizer/worker/evm"
	imx_worker "github.com/blocklords/gosds/categorizer/worker/imx"
	"github.com/blocklords/gosds/common/data_type/key_value"
	static_abi "github.com/blocklords/gosds/static/abi"
	"github.com/blocklords/gosds/static/network"

	"github.com/blocklords/gosds/app/account"
	"github.com/blocklords/gosds/app/argument"
	"github.com/blocklords/gosds/app/configuration"
	"github.com/blocklords/gosds/security/vault"

	"github.com/blocklords/gosds/app/remote"

	"github.com/blocklords/gosds/app/remote/message"

	"github.com/blocklords/gosds/app/service"

	"github.com/blocklords/gosds/db"

	"github.com/blocklords/gosds/app/controller"
)

var log_parse_in chan evm_worker.RequestLogParse = nil
var log_parse_out chan evm_worker.ReplyLogParse = nil

var static_socket *remote.Socket

var imx_manager *imx.Manager = nil
var evm_managers key_value.KeyValue

// Manages the EVM based smartcontracts on a certain blockchain
func run_evm_manager(db_con *db.Database, network *network.Network) {
	// smartcontract.GetAll() is the first database connection.
	// therefore it checks database liveness.
	smartcontracts, err := smartcontract.GetAllByNetworkId(db_con, network.Id)
	if err != nil {
		panic(`error to fetch all categorized smartcontracts. received database error: ` + err.Error() + ` for network id ` + network.Id)
	}

	manager := evm_worker.NewManager(network, log_parse_in, log_parse_out)
	manager.Run()

	workers := make(evm_worker.EvmWorkers, len(smartcontracts))
	for i, smartcontract := range smartcontracts {
		parent := worker.New(db_con, smartcontract)

		remote_abi, err := static_abi.Get(static_socket, smartcontract.NetworkId, smartcontract.Address)
		if err != nil {
			panic(fmt.Errorf("failed to set the ABI from SDS Static. This is an exception. It should not happen. error: " + err.Error()))
		}
		abi, err := abi.NewAbi(remote_abi)
		if err != nil {
			panic(errors.New("failed to create a categorizer abi wrapper. error message: " + err.Error()))
		}

		worker := evm_worker.New(parent, abi, log_parse_in, log_parse_out)
		workers[i] = worker
	}

	manager.In <- workers

	evm_managers = evm_managers.Set(network.Id, manager)
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

		go imx_worker.ImxRun(db_con, sm, imx_manager)
	}
}

////////////////////////////////////////////////////////////////////
//
// Command handlers
//
////////////////////////////////////////////////////////////////////

// Saves the smartcontract in the database.
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
			go imx_worker.ImxRun(db_con, sm, imx_manager)
		} else {
			manager_raw, ok := evm_managers[sm.NetworkId]
			if !ok {
				return message.Fail("unsupported network_id")
			}
			manager := manager_raw.(*evm_worker.Manager)

			parent := worker.New(db_con, sm)

			remote_abi, err := static_abi.Get(static_socket, sm.NetworkId, sm.Address)
			if err != nil {
				return message.Fail("failed to set the ABI from SDS Static. This is an exception. It should not happen. error: " + err.Error())
			}
			abi, err := abi.NewAbi(remote_abi)
			if err != nil {
				return message.Fail("failed to create a categorizer abi wrapper. error message: " + err.Error())
			}

			worker := evm_worker.New(parent, abi, log_parse_in, log_parse_out)
			manager.In <- evm_worker.EvmWorkers{worker}
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
	greeting := `SDS Categorizer preparing... Supported command line arguments:
    --network-id=<network id>   runs the smartcontract workers for this network id only
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

	if _, err := service.New(service.SPAGHETTI, service.REMOTE); err != nil {
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

	static_socket = remote.TcpRequestSocketOrPanic(static_env, categorizer_env)

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

	evm_managers = key_value.Empty()

	log_parse_in = make(chan evm_worker.RequestLogParse)
	log_parse_out = make(chan evm_worker.ReplyLogParse)
	go evm_worker.LogParse(log_parse_in, log_parse_out)

	for _, network := range networks {
		if network.Id == imx.NETWORK_ID {
			run_imx_manager(db_con, network)
		} else {
			run_evm_manager(db_con, network)
		}
	}

	var commands = controller.CommandHandlers{
		"smartcontract_get_all": handler.GetSmartcontracts,
		"smartcontract_get":     handler.GetSmartcontract,

		"log_get_all": handler.GetLogs,

		"snapshot_get": handler.GetSnapshot,

		"smartcontract_set": smartcontract_set,
	}

	// Allowed services to connect to SDS Categorizer
	accounts := account.NewAccounts(account.NewService(developer_gateway_env))
	accounts = accounts.Add(account.NewService(bundle_env), account.NewService(log_env))
	accounts = accounts.Add(account.NewService(reader_env), account.NewService(writer_env))
	accounts = accounts.Add(account.NewService(publisher_env), account.NewService(gateway_env))

	err = controller.ReplyController(db_con, commands, categorizer_env, accounts)
	if err != nil {
		panic(err)
	}
}
