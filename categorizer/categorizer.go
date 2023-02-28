package categorizer

import (
	"fmt"

	"github.com/charmbracelet/log"

	"github.com/blocklords/gosds/blockchain/inproc"
	"github.com/blocklords/gosds/blockchain/network"
	"github.com/blocklords/gosds/categorizer/handler"
	"github.com/blocklords/gosds/categorizer/smartcontract"
	"github.com/blocklords/gosds/common/data_type/key_value"
	static_abi "github.com/blocklords/gosds/static/abi"

	"github.com/blocklords/gosds/app/account"
	"github.com/blocklords/gosds/app/configuration"

	"github.com/blocklords/gosds/app/remote"

	"github.com/blocklords/gosds/app/remote/message"

	"github.com/blocklords/gosds/app/service"

	"github.com/blocklords/gosds/db"

	"github.com/blocklords/gosds/app/controller"
)

var static_socket *remote.Socket

// Manages the EVM based smartcontracts on a certain blockchain
// todo use the blockchain/categorizer_push(network_id); defer close()
// then to categorizer_request.add_smartcontract(smartcontract)
func register_smartcontracts(db_con *db.Database, network *network.Network) {
	smartcontracts, err := smartcontract.GetAllByNetworkId(db_con, network.Id)
	if err != nil {
		panic(`error to fetch all categorized smartcontracts. received database error: ` + err.Error() + ` for network id ` + network.Id)
	}

	pusher, err := inproc.NewCategorizerPusher(network.Id)
	if err != nil {
		panic(err)
	}
	defer pusher.Close()

	static_abis := make([]*static_abi.Abi, len(smartcontracts))

	for i, smartcontract := range smartcontracts {
		remote_abi, err := static_abi.Get(static_socket, smartcontract.NetworkId, smartcontract.Address)
		if err != nil {
			panic(fmt.Errorf("failed to set the ABI from SDS Static. This is an exception. It should not happen. error: " + err.Error()))
		}
		static_abis[i] = remote_abi
	}

	request := message.Request{
		Command: "",
		Parameters: map[string]interface{}{
			"smartcontracts": smartcontracts,
			"abis":           static_abis,
		},
	}
	request_string, _ := request.ToString()

	_, err = pusher.SendMessage(request_string)
	if err != nil {
		panic(err)
	}
}

////////////////////////////////////////////////////////////////////
//
// Command handlers
//
////////////////////////////////////////////////////////////////////

// Saves the smartcontract in the database.
// then start a worker.
func smartcontract_set(db_con *db.Database, request message.Request, logger log.Logger) message.Reply {
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

	pusher, err := inproc.NewCategorizerPusher(sm.NetworkId)
	if err != nil {
		panic(err)
	}
	defer pusher.Close()

	remote_abi, err := static_abi.Get(static_socket, sm.NetworkId, sm.Address)
	if err != nil {
		return message.Fail("failed to set the ABI from SDS Static. This is an exception. It should not happen. error: " + err.Error())
	}

	smartcontracts := []*smartcontract.Smartcontract{sm}
	static_abis := []*static_abi.Abi{remote_abi}

	push := message.Request{
		Command: "",
		Parameters: map[string]interface{}{
			"smartcontracts": smartcontracts,
			"abis":           static_abis,
		},
	}
	request_string, _ := push.ToString()

	_, err = pusher.SendMessage(request_string)
	if err != nil {
		return message.Fail(err.Error())
	}

	reply := message.Reply{
		Status:     "OK",
		Message:    "",
		Parameters: key_value.Empty().Set("smartcontract", sm),
	}

	return reply
}

// Smartcontract data are parsed and stored in the database
func Run(app_config *configuration.Config, db_con *db.Database) {
	greeting := `SDS Categorizer preparing... Supported command line arguments:
    --security-debug            prints the security logs`
	println(greeting + "\n\n")

	// check for missing environment variable otherwise panic exit.
	if _, err := service.New(service.SPAGHETTI, service.SUBSCRIBE, service.REMOTE); err != nil {
		panic(err)
	}

	categorizer_env, err := service.New(service.CATEGORIZER, service.THIS)
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

	static_socket = remote.TcpRequestSocketOrPanic(static_env, categorizer_env)

	networks, err := network.GetRemoteNetworks(static_socket, network.ALL)
	if err != nil {
		panic(err)
	}

	for _, the_network := range networks {
		register_smartcontracts(db_con, the_network)
	}

	var commands = controller.CommandHandlers{
		"smartcontract_get_all": handler.GetSmartcontracts,
		"smartcontract_get":     handler.GetSmartcontract,

		"log_get_all": handler.GetLogs,

		"snapshot_get": handler.GetSnapshot,

		"smartcontract_set": smartcontract_set,
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

	reply, err := controller.NewReply(categorizer_env)
	if err != nil {
		panic(err)
	}

	if !app_config.Plain {
		err := reply.SetControllerPrivateKey()
		if err != nil {
			panic(err)
		}
	}

	go SetupSocket(db_con)

	err = reply.Run(db_con, commands)
	if err != nil {
		panic(err)
	}
}
