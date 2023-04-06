// EVM blockchain worker's manager
// For every blockchain we have one manager.
// Manager keeps the list of the smartcontract workers:
// - list of workers for up to date smartcontracts
// - list of workers for categorization outdated smartcontracts
package categorizer

import (
	"fmt"

	"github.com/blocklords/sds/app/command"
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"

	client_thread "github.com/blocklords/sds/blockchain/inproc"
	"github.com/blocklords/sds/blockchain/network"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/blockchain/evm/categorizer/old"
	"github.com/blocklords/sds/blockchain/evm/categorizer/recent"
	"github.com/blocklords/sds/blockchain/evm/categorizer/smartcontract"
	"github.com/blocklords/sds/blockchain/handler"
	categorizer_smartcontract "github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"

	zmq "github.com/pebbe/zmq4"
)

const IDLE = "idle"
const RUNNING = "running"

// Categorization of the smartcontracts on the specific EVM blockchain
type Manager struct {
	Network *network.Network // blockchain information of the manager

	old_manager           *zmq.Socket // send through this socket updated datat to old smartcontract categorizer
	recent_manager        *zmq.Socket // send through this socket updated datat to old smartcontract categorizer
	recent_request_socket *remote.Socket
	app_config            *configuration.Config // configuration used to create new sockets
	logger                log.Logger            // print the debug parameters
}

// Creates a new manager for the given EVM Network
// New manager runs in the background.
func NewManager(
	parent log.Logger,
	network *network.Network,
	app_config *configuration.Config) (*Manager, error) {

	logger, err := parent.ChildWithTimestamp("categorizer")
	if err != nil {
		return nil, fmt.Errorf("child logger: %w", err)
	}

	manager := Manager{
		Network:    network,
		logger:     logger,
		app_config: app_config,
	}

	return &manager, nil
}

// Same as Run.
//
// Run it as a goroutine. Otherwise there is no guarantee that
// manager would connect to the blockchain/client and SDS Core correctly.
//
// Because, the sockets are not thread-safe.
func (manager *Manager) Start() {
	recent_manager, err := client_thread.RecentCategorizerManagerSocket(manager.Network.Id)
	if err != nil {
		manager.logger.Fatal("new recent manager push socket", "error", err)
	}
	manager.recent_manager = recent_manager

	reply_url := client_thread.RecentCategorizerReplyEndpoint(manager.Network.Id)
	recent_request_socket, err := remote.InprocRequestSocket(reply_url, manager.logger, manager.app_config)
	if err != nil {
		manager.logger.Fatal("new recent manager push socket", "error", err)
	}
	manager.recent_request_socket = recent_request_socket

	old_manager, err := client_thread.OldCategorizerManagerSocket(manager.Network.Id)
	if err != nil {
		manager.logger.Fatal("new old manager push socket", "error", err)
	}
	manager.old_manager = old_manager

	if err := manager.start_recent(); err != nil {
		manager.logger.Fatal("new manager push socket", "error", err)
	}
	if err := manager.start_old(); err != nil {
		manager.logger.Fatal("new manager push socket", "error", err)
	}

	go manager.start_puller()
}

// The categorizer receives new smartcontracts
// to categorize from SDS Categorizer.
func (manager *Manager) start_recent() error {
	recent_manager, err := recent.NewManager(
		manager.logger,
		manager.Network,
		manager.app_config,
	)
	if err != nil {
		return fmt.Errorf("recent.NewManager: %w", err)
	}
	go recent_manager.Start()

	return nil
}

// The categorizer receives new smartcontracts
// to categorize from SDS Categorizer.
func (manager *Manager) start_old() error {
	old_manager, err := old.NewManager(
		manager.logger,
		manager.Network,
		manager.app_config,
	)
	if err != nil {
		return fmt.Errorf("new old manager: %w", err)
	}
	go old_manager.Start()

	return nil
}

// The categorizer receives new smartcontracts
// to categorize from SDS Categorizer.
func (manager *Manager) start_puller() {
	url := client_thread.CategorizerEndpoint(manager.Network.Id)
	service, err := service.InprocessFromUrl(url)
	if err != nil {
		manager.logger.Fatal("failed to create inproc service from url", "error", err)
	}
	reply, err := controller.NewPull(service, manager.logger)
	if err != nil {
		manager.logger.Fatal("failed to create pull controller", "error", err)
	}

	handlers := command.EmptyHandlers().
		Add(handler.NEW_CATEGORIZED_SMARTCONTRACTS, on_new_smartcontracts)
	err = reply.Run(handlers, manager)
	if err != nil {
		manager.logger.Fatal("failed to run reply controller", "error", err)
	}
}

// Returns the most recent block number from recent manager through the socket.
//
// Algorithm to get block number by priority
// - from blockchain
func (manager *Manager) remote_recent_block_number() (blockchain.Number, error) {
	recent_request := handler.RecentBlockHeaderRequest{}
	var reply handler.RecentBlockHeaderReply

	err := handler.RECENT_BLOCK_NUMBER.Request(manager.recent_request_socket, recent_request, &reply)
	if err != nil {
		return 0, fmt.Errorf("RemoteRequest: %w", err)
	}
	if err := reply.Validate(); err != nil {
		return 0, fmt.Errorf("reply.Validate: %w", err)
	}

	return reply.Number, nil
}

// Categorizer manager received new smartcontracts along with their ABI
func on_new_smartcontracts(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	if parameters == nil || len(parameters) < 1 {
		return message.Fail("invalid parameters were given atleast manager should be passed")
	}

	manager, ok := parameters[0].(*Manager)
	if !ok {
		return message.Fail("missing Manager in the parameters")
	}

	manager.logger.Info("add new smartcontracts to the manager")

	raw_smartcontracts, _ := request.Parameters.GetKeyValueList("smartcontracts")

	block_number, err := manager.remote_recent_block_number()
	if err != nil {
		return message.Fail("recent block number empty, its unexpected: " + err.Error())
	}

	new_workers := make(smartcontract.EvmWorkers, len(raw_smartcontracts))
	for i, raw_sm := range raw_smartcontracts {
		sm, _ := categorizer_smartcontract.New(raw_sm)
		manager.logger.Info("add a new worker", "number", i+1, "total", len(new_workers))
		new_workers[i] = smartcontract.New(sm, nil)
	}

	manager.logger.Info("information about workers", "block_number", block_number, "amount of workers", len(new_workers))

	old_workers, recent_workers := new_workers.Sort().Split(block_number)
	manager.logger.Info("splitting to old and new workers", "old amount", len(old_workers), "new amount", len(recent_workers))

	if len(old_workers) > 0 {
		err := manager.push_old_workers(old_workers)
		return message.Fail("push_old_workers: " + err.Error())
	}

	if len(recent_workers) > 0 {
		err := manager.push_recent_workers(recent_workers)
		return message.Fail("push_recent_workers: " + err.Error())
	}

	return message.Reply{
		Status:     message.OK,
		Message:    "",
		Parameters: key_value.Empty(),
	}
}

func (manager *Manager) push_old_workers(workers smartcontract.EvmWorkers) error {
	push := handler.PushNewSmartcontracts{
		Smartcontracts: workers.GetSmartcontracts(),
	}
	err := handler.NEW_CATEGORIZED_SMARTCONTRACTS.Push(manager.old_manager, push)
	if err != nil {
		return fmt.Errorf("failed to send to old categorizer: %w", err)
	}

	return nil
}

func (manager *Manager) push_recent_workers(workers smartcontract.EvmWorkers) error {
	push := handler.PushNewSmartcontracts{
		Smartcontracts: workers.GetSmartcontracts(),
	}
	err := handler.NEW_CATEGORIZED_SMARTCONTRACTS.Push(manager.recent_manager, push)
	if err != nil {
		return fmt.Errorf("failed to send to old categorizer: %w", err)
	}

	return nil
}
