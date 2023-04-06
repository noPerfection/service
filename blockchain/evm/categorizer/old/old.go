// EVM blockchain worker's manager
// For every blockchain we have one manager.
// Manager keeps the list of the smartcontract workers:
// - list of workers for up to date smartcontracts
// - list of workers for categorization outdated smartcontracts
package old

import (
	"fmt"
	"strings"
	"sync"

	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/categorizer"

	"time"

	client_thread "github.com/blocklords/sds/blockchain/inproc"
	"github.com/blocklords/sds/blockchain/network"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/blockchain/evm/abi"
	"github.com/blocklords/sds/blockchain/evm/categorizer/smartcontract"
	"github.com/blocklords/sds/blockchain/handler"
	categorizer_event "github.com/blocklords/sds/categorizer/event"
	categorizer_command "github.com/blocklords/sds/categorizer/handler"
	categorizer_smartcontract "github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	static_command "github.com/blocklords/sds/static/handler"

	"github.com/blocklords/sds/app/remote/message"
	spaghetti_log "github.com/blocklords/sds/blockchain/event"
	zmq "github.com/pebbe/zmq4"

	"github.com/blocklords/sds/app/remote"
)

const IDLE = "idle"
const RUNNING = "running"

// Categorization of the smartcontracts on the specific EVM blockchain
type Manager struct {
	Network *network.Network // blockchain information of the manager

	pusher                *zmq.Socket // send through this socket updated data to SDS Core
	recent_request_socket *remote.Socket
	recent_manager        *zmq.Socket           // send
	static                *remote.Socket        // return the abi from static for decoding event logs
	app_config            *configuration.Config // configuration used to create new sockets
	logger                log.Logger            // print the debug parameters
	old_categorizers      OldWorkerGroups       // smartcontracts to categorize from archived nodes
}

// Creates a new manager for the given EVM Network
// New manager runs in the background.
func NewManager(l log.Logger, n *network.Network, c *configuration.Config) (*Manager, error) {
	logger, err := l.ChildWithTimestamp("old")
	if err != nil {
		return nil, fmt.Errorf("child logger: %w", err)
	}

	manager := Manager{
		Network:          n,
		old_categorizers: make(OldWorkerGroups, 0),
		logger:           logger,
		app_config:       c,
	}

	return &manager, nil
}

// Returns all smartcontracts
// those that are synced from archived blocks
// and the ones that are syncing from the up-to-date blocks.
func (manager *Manager) GetSmartcontracts() []categorizer_smartcontract.Smartcontract {
	smartcontracts := make([]categorizer_smartcontract.Smartcontract, 0)

	for _, group := range manager.old_categorizers {
		smartcontracts = append(smartcontracts, group.workers.GetSmartcontracts()...)
	}

	return smartcontracts
}

// Returns all smartcontract address
// those that are synced from archived blocks
// and the ones that are syncing from the up-to-date blocks.
func (manager *Manager) GetSmartcontractAddresses() []string {
	addresses := make([]string, 0)

	for _, group := range manager.old_categorizers {
		addresses = append(addresses, group.workers.GetSmartcontractAddresses()...)
	}

	return addresses
}

// Returns the most recent block number that manager synced to.
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

// Same as Run.
//
// Run it as a goroutine. Otherwise there is no guarantee that
// manager would connect to the blockchain/client and SDS Core correctly.
//
// Because, the sockets are not thread-safe.
func (manager *Manager) Start() {
	static_env, err := service.Inprocess(service.STATIC)
	if err != nil {
		manager.logger.Fatal("no inproc service configuration", "service type", service.STATIC, "error", err)
	}
	static_socket, err := remote.InprocRequestSocket(static_env.Url(), manager.logger, manager.app_config)
	if err != nil {
		manager.logger.Fatal("remote.InprocRequest", "url", static_env.Url(), "error", err)
	}
	manager.static = static_socket

	sock, err := zmq.NewSocket(zmq.PULL)
	if err != nil {
		manager.logger.Fatal("new manager pull socket", "message", err)
	}

	url := client_thread.OldCategorizerEndpoint(manager.Network.Id)
	if err := sock.Connect(url); err != nil {
		manager.logger.Fatal("trying to create categorizer for network id %s: %v", manager.Network.Id, err)
	}

	manager.logger.Info("waiting for the messages at", "url", url)

	categorizer_pusher, err := categorizer.NewCategorizerPusher()
	if err != nil {
		manager.logger.Fatal("v.NewCategorizerPusher", "error", err)
	}
	manager.pusher = categorizer_pusher

	recent_manager, err := client_thread.RecentCategorizerManagerSocket(manager.Network.Id)
	if err != nil {
		manager.logger.Fatal("client_thread.RecentCategorizerManagerSocket", "error", err)
	}
	manager.recent_manager = recent_manager

	reply_url := client_thread.RecentCategorizerReplyEndpoint(manager.Network.Id)
	recent_request_socket, err := remote.InprocRequestSocket(reply_url, manager.logger, manager.app_config)
	if err != nil {
		manager.logger.Fatal("new recent manager push socket", "error", err)
	}
	manager.recent_request_socket = recent_request_socket

	for {
		// Wait for reply.
		msgs, _ := sock.RecvMessage(0)
		request, _ := message.ParseRequest(msgs)

		if request.Command == handler.NEW_CATEGORIZED_SMARTCONTRACTS.String() {
			manager.on_new_smartcontracts(request.Parameters)
		}
	}
}

// Categorizer manager received new smartcontracts along with their ABI
func (manager *Manager) on_new_smartcontracts(parameters key_value.KeyValue) {
	var mu sync.Mutex
	manager.logger.Info("add new smartcontracts to the manager")

	raw_smartcontracts, _ := parameters.GetKeyValueList("smartcontracts")

	old_workers := make(smartcontract.EvmWorkers, len(raw_smartcontracts))
	for i, raw_sm := range raw_smartcontracts {
		sm, _ := categorizer_smartcontract.New(raw_sm)

		mu.Lock()
		var sm_req static_command.GetSmartcontractRequest = sm.SmartcontractKey
		var sm_reply static_command.GetSmartcontractReply
		err := static_command.GET_SMARTCONTRACT.Request(manager.static, sm_req, &sm_reply)
		if err != nil {
			mu.Unlock()
			manager.logger.Fatal("remote static smartcontract get", "error", err)
		}

		req := static_command.GetAbiRequest{
			Id: sm_reply.AbiId,
		}
		var abi_data static_command.GetAbiReply
		err = static_command.GET_ABI.Request(manager.static, req, &abi_data)
		mu.Unlock()
		if err != nil {
			manager.logger.Fatal("remote static abi get", "error", err)
		}

		cat_abi, err := abi.NewFromStatic(&abi_data)
		if err != nil {
			manager.logger.Fatal("failed to decode", "index", i, "smartcontract", sm.SmartcontractKey.Address, "errr", err)
		}
		manager.logger.Info("add a new worker", "number", i+1, "total", len(old_workers))
		old_workers[i] = smartcontract.New(sm, cat_abi)
	}

	old_block_number := old_workers.EarliestBlockNumber()

	manager.logger.Info("old workers information", "earliest_block_number", old_block_number)

	if len(old_workers) > 0 {
		group := manager.old_categorizers.FirstGroupGreaterThan(old_block_number)
		if group == nil {
			manager.logger.Info("create a new group of old workers")
			group = NewGroup(old_block_number, old_workers)
			manager.old_categorizers = append(manager.old_categorizers, group)
			go manager.categorize_old_smartcontracts(group)
		} else {
			manager.logger.Info("add to the existing group")
			group.add_workers(old_workers)
		}
	}
}

// Categorization of the smartcontracts that are super old.
// Categorize them with the data from archived nodes.
//
// Get List of smartcontract addresses
// Get Log for the smartcontracts.
func (manager *Manager) categorize_old_smartcontracts(group *OldWorkerGroup) {
	old_logger, err := manager.logger.ChildWithTimestamp("old_logger_" + time.Now().String())
	if err != nil {
		manager.logger.Fatal("failed to create child logger", "message", err)
	}

	url := client_thread.ClientEndpoint(manager.Network.Id)
	blockchain_client_socket, err := remote.InprocRequestSocket(url, old_logger, manager.app_config)
	if err != nil {
		manager.logger.Fatal("remote.InprocRequest", "url", url, "error", err)
	}
	defer blockchain_client_socket.Close()

	old_logger.Info("starting categorization of old smartcontracts.", "blockchain client manager", url)

	for {
		block_number_from := group.block_number.Increment()
		addresses := group.workers.GetSmartcontractAddresses()

		old_logger.Info("fetch from blockchain client manager logs", "block_number", block_number_from, "addresses", addresses)

		req_parameters := handler.FilterLog{
			BlockFrom: block_number_from,
			Addresses: addresses,
		}

		var parameters handler.LogFilterReply
		err = handler.FILTER_LOG_COMMAND.Request(blockchain_client_socket, req_parameters, &parameters)

		if err != nil {
			old_logger.Warn("SKIP, blockchain manager returned an error for block number %d and addresses %v: %w", block_number_from, addresses, err)
			time.Sleep(time.Second)
			continue
		}

		block_to, _ := blockchain.NewHeader(parameters.BlockTo, parameters.BlockTo)
		if len(parameters.RawLogs) > 0 {
			block_to = spaghetti_log.RecentBlock(parameters.RawLogs)
		}
		old_logger.Info("fetched from blockchain client manager", "logs amount", len(parameters.RawLogs), "smartcontract address", addresses, "block_to", block_to)

		decoded_logs := make([]categorizer_event.Log, 0)

		// decode the logs
		for _, raw_log := range parameters.RawLogs {
			for _, worker := range group.workers {
				if worker.Smartcontract.SmartcontractKey.Address != raw_log.Transaction.SmartcontractKey.Address {
					continue
				}

				decoded_log, err := worker.DecodeLog(&raw_log)
				if err != nil {
					old_logger.Fatal("worker.DecodeLog", "smartcontract", worker.Smartcontract.SmartcontractKey.Address, "message", err)
				}

				decoded_logs = append(decoded_logs, decoded_log)
			}
		}

		// update the categorization state for the smartcontract
		smartcontracts := group.workers.GetSmartcontracts()
		for _, smartcontract := range smartcontracts {
			new_block, _ := blockchain.NewHeader(uint64(block_to.Number), uint64(block_to.Timestamp))

			for _, decoded_log := range decoded_logs {
				if strings.EqualFold(decoded_log.SmartcontractKey.Address, smartcontract.SmartcontractKey.Address) {
					new_block = decoded_log.BlockHeader
				}
			}
			smartcontract.SetBlockHeader(new_block)
		}

		old_logger.Info("notify SDS Categorizer about update", "block_number_from", block_number_from, "block_number_to", parameters.BlockTo)

		// now we send the categorized smartcontracts and logs information
		// to SDS Categorizer, so that SDS Categorizer will update its Database
		request := categorizer_command.PushCategorization{
			Smartcontracts: smartcontracts,
			Logs:           decoded_logs,
		}
		err = categorizer_command.CATEGORIZATION.Push(manager.pusher, request)
		if err != nil {
			old_logger.Fatal("send to SDS Categorizer", "error", err)
		}

		recent_block_number, err := manager.remote_recent_block_number()
		if err != nil {
			old_logger.Fatal("remote_recent_block_number", "error", err)
		}
		left := recent_block_number.Value() - parameters.BlockTo
		old_logger.Info("categorized certain blocks", "block_number_left", left, "block_number_to", parameters.BlockTo, "subscribed", recent_block_number)
		group.block_number = block_to.Number

		if parameters.BlockTo >= recent_block_number.Value() {
			old_logger.Info("catched the recent blocks")
			manager.push_recent_workers(group.workers)
			break
		}

		// do not pressure the backend
		time.Sleep(time.Second)
	}
	// delete the categorizer group
	manager.old_categorizers = manager.old_categorizers.Delete(group)

	old_logger.Info("finished!")
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
