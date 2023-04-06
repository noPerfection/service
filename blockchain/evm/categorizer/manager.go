// EVM blockchain worker's manager
// For every blockchain we have one manager.
// Manager keeps the list of the smartcontract workers:
// - list of workers for up to date smartcontracts
// - list of workers for categorization outdated smartcontracts
package categorizer

import (
	"fmt"

	"github.com/blocklords/sds/app/log"

	client_thread "github.com/blocklords/sds/blockchain/inproc"
	"github.com/blocklords/sds/blockchain/network"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/blockchain/evm/categorizer/old"
	"github.com/blocklords/sds/blockchain/evm/categorizer/recent"
	"github.com/blocklords/sds/blockchain/evm/categorizer/smartcontract"
	"github.com/blocklords/sds/blockchain/handler"
	categorizer_smartcontract "github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type"
	"github.com/blocklords/sds/common/data_type/key_value"

	"github.com/blocklords/sds/app/remote/message"
	zmq "github.com/pebbe/zmq4"
)

const IDLE = "idle"
const RUNNING = "running"

// Categorization of the smartcontracts on the specific EVM blockchain
type Manager struct {
	Network *network.Network // blockchain information of the manager

	old_manager         *zmq.Socket              // send through this socket updated datat to old smartcontract categorizer
	recent_manager      *zmq.Socket              // send through this socket updated datat to old smartcontract categorizer
	app_config          *configuration.Config    // configuration used to create new sockets
	logger              log.Logger               // print the debug parameters
	recent_workers      smartcontract.EvmWorkers // up-to-date smartcontracts consumes subscribed_blocks
	subscribed_blocks   data_type.Queue          // we keep recent blocks from blockchain
	recent_block_number blockchain.Number
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
		Network:           network,
		subscribed_blocks: *data_type.NewQueue(),
		recent_workers:    make(smartcontract.EvmWorkers, 0),
		logger:            logger,
		app_config:        app_config,
	}

	return &manager, nil
}

// Same as Run.
//
// Run it as a goroutine. Otherwise there is no guarantee that
// manager would connect to the blockchain/client and SDS Core correctly.
//
// Because, the sockets are not thread-safe.
func (manager *Manager) Start(categorizer_pusher *zmq.Socket) {
	recent_manager, err := client_thread.RecentCategorizerManagerSocket(manager.Network.Id)
	if err != nil {
		manager.logger.Fatal("new recent manager push socket", "error", err)
	}
	manager.recent_manager = recent_manager

	old_manager, err := client_thread.OldCategorizerManagerSocket(manager.Network.Id)
	if err != nil {
		manager.logger.Fatal("new old manager push socket", "error", err)
	}
	manager.old_manager = old_manager

	if err := manager.start_recent(categorizer_pusher); err != nil {
		manager.logger.Fatal("new manager push socket", "error", err)
	}
	if err := manager.start_old(categorizer_pusher); err != nil {
		manager.logger.Fatal("new manager push socket", "error", err)
	}

	go manager.start_puller()
}

// The categorizer receives new smartcontracts
// to categorize from SDS Categorizer.
func (manager *Manager) start_recent(categorizer_pusher *zmq.Socket) error {
	recent_manager, err := recent.NewManager(
		manager.logger,
		manager.Network,
		categorizer_pusher,
		manager.old_manager,
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
func (manager *Manager) start_old(categorizer_pusher *zmq.Socket) error {
	old_manager, err := old.NewManager(
		manager.logger,
		manager.Network,
		categorizer_pusher,
		manager.recent_manager,
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
	sock, err := zmq.NewSocket(zmq.PULL)
	if err != nil {
		manager.logger.Fatal("new manager pull socket", "message", err)
	}

	url := client_thread.CategorizerEndpoint(manager.Network.Id)
	if err := sock.Connect(url); err != nil {
		manager.logger.Fatal("trying to create categorizer for network id %s: %v", manager.Network.Id, err)
	}

	manager.logger.Info("waiting for the messages at", "url", url)

	for {
		// Wait for reply.
		msgs, _ := sock.RecvMessage(0)
		request, _ := message.ParseRequest(msgs)

		if request.Command == handler.NEW_CATEGORIZED_SMARTCONTRACTS.String() {
			manager.on_new_smartcontracts(request.Parameters)
		} else if request.Command == handler.RECENT_BLOCK_NUMBER.String() {
			manager.on_recent_block_number(request.Parameters)
		}
	}
}

// Categorizer manager received new smartcontracts along with their ABI
func (manager *Manager) on_recent_block_number(parameters key_value.KeyValue) {
	manager.logger.Info("add new smartcontracts to the manager")

	var recent_request handler.RecentBlockHeaderRequest
	err := parameters.ToInterface(&recent_request)
	if err != nil {
		manager.logger.Fatal("failed to receive recent block number", "error", err)
	}
	if err := recent_request.Validate(); err != nil {
		manager.logger.Fatal("recent_request.Validate", "error", err)
	}

	manager.recent_block_number = recent_request.Number
}

// Categorizer manager received new smartcontracts along with their ABI
func (manager *Manager) on_new_smartcontracts(parameters key_value.KeyValue) {
	manager.logger.Info("add new smartcontracts to the manager")

	raw_smartcontracts, _ := parameters.GetKeyValueList("smartcontracts")

	// make sure that it works with the recent
	block_number := manager.recent_block_number
	if err := block_number.Validate(); err != nil {
		manager.logger.Fatal("recent block number empty, its unexpected")
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
		manager.logger.Fatal("push_old_workers", "error", err)
	}

	if len(recent_workers) > 0 {
		err := manager.push_recent_workers(recent_workers)
		manager.logger.Fatal("push_recent_workers", "error", err)
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
