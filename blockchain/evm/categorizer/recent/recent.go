// EVM blockchain worker's manager
// For every blockchain we have one manager.
// Manager keeps the list of the smartcontract workers:
// - list of workers for up to date smartcontracts
// - list of workers for categorization outdated smartcontracts
package recent

import (
	"fmt"
	"strings"
	"sync"

	"github.com/blocklords/sds/app/command"
	"github.com/blocklords/sds/app/controller"
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
	"github.com/blocklords/sds/common/data_type"
	"github.com/blocklords/sds/common/data_type/key_value"
	static_command "github.com/blocklords/sds/static/handler"

	"github.com/blocklords/sds/app/remote/message"
	spaghetti_log "github.com/blocklords/sds/blockchain/event"
	spaghetti_block "github.com/blocklords/sds/blockchain/evm/block"
	zmq "github.com/pebbe/zmq4"

	"github.com/blocklords/sds/app/remote"
)

const IDLE = "idle"
const RUNNING = "running"

// Categorization of the smartcontracts on the specific EVM blockchain
type Manager struct {
	Network *network.Network // blockchain information of the manager

	static            *remote.Socket           // return the abi from static for decoding event logs
	app_config        *configuration.Config    // configuration used to create new sockets
	logger            log.Logger               // print the debug parameters
	workers           smartcontract.EvmWorkers // up-to-date smartcontracts consumes subscribed_blocks
	subscribed_blocks data_type.Queue          // we keep recent blocks from blockchain

	pusher *zmq.Socket // send through this socket updated datat to SDS Core
}

// Creates a new manager for the given EVM Network
// New manager runs in the background.
func NewManager(l log.Logger, n *network.Network, c *configuration.Config) (*Manager, error) {
	logger, err := l.ChildWithTimestamp("recent")
	if err != nil {
		return nil, fmt.Errorf("child logger: %w", err)
	}

	manager := Manager{
		Network:           n,
		subscribed_blocks: *data_type.NewQueue(),
		workers:           make(smartcontract.EvmWorkers, 0),
		logger:            logger,
		app_config:        c,
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
	static_env, err := service.Inprocess(service.STATIC)
	if err != nil {
		manager.logger.Fatal("no inproc service configuration", "service type", service.STATIC, "error", err)
	}
	static_socket, err := remote.InprocRequestSocket(static_env.Url(), manager.logger, manager.app_config)
	if err != nil {
		manager.logger.Fatal("remote.InprocRequest", "url", static_env.Url(), "error", err)
	}
	manager.static = static_socket

	categorizer_pusher, err := categorizer.NewCategorizerPusher()
	if err != nil {
		manager.logger.Fatal("new old manager push socket", "error", err)
	}
	manager.pusher = categorizer_pusher

	manager.logger.Info("starting categorization")
	go manager.queue_recent_blocks()
}

// Returns the most recent block number.
// If there is no block number then it returns error
func on_recent_block_number(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	var request_parameters handler.RecentBlockHeaderRequest
	err := request.Parameters.ToInterface(&request_parameters)
	if err != nil {
		return message.Fail("invalid request parameters:" + err.Error())
	}
	if err := request_parameters.Validate(); err != nil {
		return message.Fail("request parameters validate:" + err.Error())
	}

	if parameters == nil || len(parameters) < 1 {
		return message.Fail("invalid parameters were given atleast manager should be passed")
	}

	manager, ok := parameters[0].(*Manager)
	if !ok {
		return message.Fail("missing Manager in the parameters")
	}
	block_number := manager.recent_block_number()
	if err := block_number.Validate(); err != nil {
		return message.Fail("missing recent block number")
	}

	reply, err := blockchain.NewHeader(block_number.Value(), block_number.Value())
	if err != nil {
		return message.Fail("blockchain.NewHeader:" + err.Error())
	}

	var reply_parameters = reply
	message_reply, err := command.Reply(&reply_parameters)
	if err != nil {
		return message.Fail("blockchain.NewHeader:" + err.Error())
	}

	return message_reply
}

// The categorizer receives new smartcontracts
// to categorize from SDS Categorizer.
func (manager *Manager) start_puller() {
	url := client_thread.RecentCategorizerEndpoint(manager.Network.Id)
	service, err := service.InprocessFromUrl(url)
	if err != nil {
		manager.logger.Fatal("failed to create inproc service from url", "error", err)
	}
	reply, err := controller.NewPull(service, manager.logger)
	if err != nil {
		manager.logger.Fatal("failed to create pull controller", "error", err)
	}

	handlers := command.EmptyHandlers().Add(handler.NEW_CATEGORIZED_SMARTCONTRACTS, on_new_smartcontracts)
	err = reply.Run(handlers, manager)
	if err != nil {
		manager.logger.Fatal("failed to run reply controller", "error", err)
	}
}

func (manager *Manager) start_req_reply() {
	url := client_thread.RecentCategorizerReplyEndpoint(manager.Network.Id)
	service, err := service.InprocessFromUrl(url)
	if err != nil {
		manager.logger.Fatal("failed to create inproc service from url", "error", err)
	}
	reply, err := controller.NewReply(service, manager.logger)
	if err != nil {
		manager.logger.Fatal("failed to create reply controller", "error", err)
	}

	handlers := command.EmptyHandlers().Add(handler.RECENT_BLOCK_NUMBER, on_recent_block_number)
	err = reply.Run(handlers, manager)
	if err != nil {
		manager.logger.Fatal("failed to run reply controller", "error", err)
	}
}

// Returns the recent block number.
//
// If we have new block to consume, then we pick the first.
// If we don't have new blocks but we have some recent
// workers then we get the first recent worker's number.
//
// Otherwise we returns 0.
func (manager *Manager) recent_block_number() blockchain.Number {
	if !manager.subscribed_blocks.IsEmpty() {
		recent_block_number := manager.subscribed_blocks.First().(*spaghetti_block.Block).Header.Number
		return recent_block_number
	}

	if num := manager.workers.EarliestBlockNumber(); num != 0 {
		return num
	}

	return 0
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

	var mu sync.Mutex
	manager.logger.Info("add new smartcontracts to the manager")

	raw_smartcontracts, _ := request.Parameters.GetKeyValueList("smartcontracts")

	block_number := manager.recent_block_number()
	if err := block_number.Validate(); err != nil {
		return message.Fail("recent block number empty, its unexpected: " + err.Error())
	}

	workers := make(smartcontract.EvmWorkers, len(raw_smartcontracts))
	for i, raw_sm := range raw_smartcontracts {
		sm, _ := categorizer_smartcontract.New(raw_sm)

		mu.Lock()
		var sm_req = sm.SmartcontractKey
		var sm_reply static_command.GetSmartcontractReply
		err := static_command.GET_SMARTCONTRACT.Request(manager.static, sm_req, &sm_reply)
		if err != nil {
			mu.Unlock()
			return message.Fail("remote static smartcontract get: " + err.Error())
		}

		req := static_command.GetAbiRequest{
			Id: sm_reply.AbiId,
		}
		var abi_data static_command.GetAbiReply
		err = static_command.GET_ABI.Request(manager.static, req, &abi_data)
		mu.Unlock()
		if err != nil {
			return message.Fail("remote static abi get: " + err.Error())
		}

		cat_abi, err := abi.NewFromStatic(&abi_data)
		if err != nil {
			return message.Fail("failed to decode static abi into categorizer abi: " + err.Error())
		}
		manager.logger.Info("add a new worker", "number", i+1, "total", len(workers))
		workers[i] = smartcontract.New(sm, cat_abi)
	}

	manager.logger.Info("information about workers", "block_number", block_number, "amount of workers", len(workers))

	workers = workers.Sort()

	manager.workers = append(manager.workers, workers...)

	return message.Reply{
		Status:     message.OK,
		Message:    "",
		Parameters: key_value.Empty(),
	}
}

// categorizes each consumed block against manager.Workers
func (manager *Manager) categorize() {
	logger, err := manager.logger.ChildWithTimestamp("recent")
	if err != nil {
		manager.logger.Fatal("failed to create child logger", "error", err)
	}

	logger.Info("starting to consume subscribed blocks...")

	for {
		time.Sleep(time.Second * time.Duration(1))

		if len(manager.workers) == 0 || manager.subscribed_blocks.IsEmpty() {
			continue
		}

		// consume each block by workers
		for {
			raw_block := manager.subscribed_blocks.Pop().(*spaghetti_block.Block)

			decoded_logs := make([]categorizer_event.Log, 0)

			// decode the logs
			for _, raw_log := range raw_block.RawLogs {
				for _, worker := range manager.workers {
					if worker.Smartcontract.SmartcontractKey.Address != raw_log.Transaction.SmartcontractKey.Address {
						continue
					}

					decoded_log, err := worker.DecodeLog(&raw_log)
					if err != nil {
						logger.Error("worker.DecodeLog", "smartcontract", worker.Smartcontract.SmartcontractKey.Address, "message", err)
						continue
					}

					decoded_logs = append(decoded_logs, decoded_log)
				}
			}

			// update the categorization state for the smartcontract
			smartcontracts := manager.workers.GetSmartcontracts()
			for _, smartcontract := range smartcontracts {
				new_block := raw_block.Header

				for _, decoded_log := range decoded_logs {
					if strings.EqualFold(decoded_log.SmartcontractKey.Address, smartcontract.SmartcontractKey.Address) {
						new_block = decoded_log.BlockHeader
					}
				}
				smartcontract.SetBlockHeader(new_block)
			}

			logger.Info("send a notification to SDS Categorizer", "logs_amount", len(decoded_logs))

			request := categorizer_command.PushCategorization{
				Smartcontracts: smartcontracts,
				Logs:           decoded_logs,
			}
			err = categorizer_command.CATEGORIZATION.Push(manager.pusher, request)
			if err != nil {
				logger.Fatal("sending notification to SDS Categorizer", "message", err)
			}

			if len(manager.workers) == 0 || manager.subscribed_blocks.IsEmpty() {
				break
			}
		}
	}
}

// Returns the most recent block number that manager synced to.
//
// Algorithm to get block number by priority
// - from blockchain
func (manager *Manager) remote_recent_block_number(client_socket *remote.Socket) (blockchain.Number, error) {
	recent_request := handler.RecentBlockHeaderRequest{}
	var recent_reply handler.RecentBlockHeaderReply

	err := handler.RECENT_BLOCK_NUMBER.Request(client_socket, recent_request, &recent_reply)
	if err != nil {
		return 0, fmt.Errorf("RemoteRequest: %w", err)
	}

	return recent_reply.Number, nil
}

// returns the block's logs
func (manager *Manager) get_filtered_block(sub_logger log.Logger, client_socket *remote.Socket, block_number blockchain.Number) (*spaghetti_block.Block, error) {
	req_parameters := handler.FilterLog{
		BlockFrom: block_number,
		Addresses: []string{},
	}
	var parameters handler.LogFilterReply

	err := handler.FILTER_LOG_COMMAND.Request(client_socket, req_parameters, &parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to get the log filters: %w", err)
	}

	if len(parameters.RawLogs) == 0 {
		block_header, _ := blockchain.NewHeader(
			block_number.Value(),
			block_number.Value(),
		)
		return &spaghetti_block.Block{
			NetworkId: manager.Network.Id,
			Header:    block_header,
			RawLogs:   parameters.RawLogs,
		}, nil
	}

	block_to := spaghetti_log.RecentBlock(parameters.RawLogs)
	new_block := spaghetti_block.Block{
		NetworkId: manager.Network.Id,
		Header:    block_to,
		RawLogs:   parameters.RawLogs,
	}

	return &new_block, nil
}

// We start to consume the block information from SDS Spaghetti
// And put it in the queue.
// The worker will start to consume them one by one.
func (manager *Manager) queue_recent_blocks() {
	sub_logger, err := manager.logger.ChildWithoutReport("recent_block_queue")
	if err != nil {
		manager.logger.Fatal("failed to create child log", "error", err)
	}

	url := client_thread.ClientEndpoint(manager.Network.Id)
	blockchain_client_socket, err := remote.InprocRequestSocket(url, sub_logger, manager.app_config)
	if err != nil {
		manager.logger.Fatal("remote.InprocRequest", "url", url, "error", err)
	}
	sub_logger.Info("pause 10 seconds before each log filter")

	block_number, err := manager.remote_recent_block_number(blockchain_client_socket)
	if err != nil {
		sub_logger.Fatal("failed to get remote recent_block_number:", "error", err)
	} else if err := block_number.Validate(); err != nil {
		manager.logger.Fatal("recent_block_number.Validate", "error", err)
	}

	puller_off := true

	for {
		if manager.subscribed_blocks.IsFull() {
			sub_logger.Warn("subscribed block is full. Start to consume them [trying in 10 seconds]", "message", err)

			time.Sleep(10 * time.Second)
			continue
		}

		// get the recent block
		// if its empty then get the new one
		block, err := manager.get_filtered_block(sub_logger, blockchain_client_socket, block_number)
		if err != nil {
			sub_logger.Warn("manager.get_filtered_block", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		if len(block.RawLogs) == 0 {
			block_number = block_number.Increment()
			sub_logger.Warn("no logs were found, sleep and continue from next block", "block_number", block_number)
			time.Sleep(10 * time.Second)
			continue
		}

		// we already accumulated the logs
		if block.Header.Number == block_number {
			block_number = block_number.Increment()
			sub_logger.Warn("reached out to the most recent logs", "block_number", block_number)
			time.Sleep(10 * time.Second)
			continue
		}

		sub_logger.Info("add a block to consume", "block_parameter", block.Header, "event log amount", len(block.RawLogs))
		manager.subscribed_blocks.Push(*block)

		if puller_off {
			go manager.start_puller()
			go manager.start_req_reply()
			go manager.categorize()
			puller_off = false
		}

		block_number = block.Header.Number.Increment()
		time.Sleep(10 * time.Second)
	}
}
