// EVM blockchain worker's manager
// For every blockchain we have one manager.
// Manager keeps the list of the smartcontract workers:
// - list of workers for up to date smartcontracts
// - list of workers for categorization outdated smartcontracts
package categorizer

import (
	"fmt"
	"strings"
	"sync"

	app_log "github.com/blocklords/gosds/app/log"
	"github.com/blocklords/gosds/categorizer"
	"github.com/charmbracelet/log"

	"time"

	blockchain_proc "github.com/blocklords/gosds/blockchain/inproc"
	"github.com/blocklords/gosds/blockchain/network"

	"github.com/blocklords/gosds/blockchain/evm/abi"
	"github.com/blocklords/gosds/blockchain/evm/categorizer/smartcontract"
	categorizer_event "github.com/blocklords/gosds/categorizer/event"
	categorizer_smartcontract "github.com/blocklords/gosds/categorizer/smartcontract"
	"github.com/blocklords/gosds/common/data_type"
	"github.com/blocklords/gosds/common/data_type/key_value"
	static_abi "github.com/blocklords/gosds/static/abi"

	"github.com/blocklords/gosds/app/remote/message"
	spaghetti_log "github.com/blocklords/gosds/blockchain/event"
	spaghetti_block "github.com/blocklords/gosds/blockchain/evm/block"
	zmq "github.com/pebbe/zmq4"

	"github.com/blocklords/gosds/app/remote"
)

const IDLE = "idle"
const RUNNING = "running"

// Categorization of the smartcontracts on the specific EVM blockchain
type Manager struct {
	pusher  *zmq.Socket
	Network *network.Network

	logger log.Logger

	old_categorizers OldWorkerGroups

	current_workers smartcontract.EvmWorkers

	subscribed_blocks data_type.Queue
}

// Creates a new manager for the given EVM Network
// New manager runs in the background.
func NewManager(logger log.Logger, network *network.Network) *Manager {
	categorizer_logger := app_log.Child(logger, "categorizer")

	manager := Manager{
		Network: network,

		old_categorizers: make(OldWorkerGroups, 0),

		subscribed_blocks: *data_type.NewQueue(),

		// consumes the data from the subscribed blocks
		current_workers: make(smartcontract.EvmWorkers, 0),

		logger: categorizer_logger,
	}

	return &manager
}

// Returns all smartcontracts from all types of workers
func (manager *Manager) GetSmartcontracts() []*categorizer_smartcontract.Smartcontract {
	smartcontracts := make([]*categorizer_smartcontract.Smartcontract, 0)

	for _, group := range manager.old_categorizers {
		smartcontracts = append(smartcontracts, group.workers.GetSmartcontracts()...)
	}

	smartcontracts = append(smartcontracts, manager.current_workers.GetSmartcontracts()...)

	return smartcontracts
}

func (manager *Manager) GetSmartcontractAddresses() []string {
	addresses := make([]string, 0)

	for _, group := range manager.old_categorizers {
		addresses = append(addresses, group.workers.GetSmartcontractAddresses()...)
	}

	addresses = append(addresses, manager.current_workers.GetSmartcontractAddresses()...)

	return addresses
}

// Same as Run. Run it as a goroutine
func (manager *Manager) Start() {
	manager.logger.Info("starting categorization")
	go manager.queue_recent_blocks()
	go manager.categorize_current_smartcontracts()

	sock, err := zmq.NewSocket(zmq.PULL)
	if err != nil {
		manager.logger.Fatal("new manager pull socket", "message", err)
	}

	url := blockchain_proc.CategorizerManagerUrl(manager.Network.Id)
	if err := sock.Connect(url); err != nil {
		log.Fatal("trying to create categorizer for network id %s: %v", manager.Network.Id, err)
	}

	// if there are some logs, we should broadcast them to the SDS Categorizer
	pusher, err := categorizer.NewCategorizerPusher()
	if err != nil {
		manager.logger.Fatal("create a pusher to SDS Categorizer", "message", err)
	}
	manager.pusher = pusher

	manager.logger.Info("waiting for the messages at", "url", url)

	for {
		// Wait for reply.
		msgs, _ := sock.RecvMessage(0)
		request, _ := message.ParseRequest(msgs)

		if request.Command == "new-smartcontracts" {
			manager.new_smartcontracts(request.Parameters)
		}
	}
}

// Categorizer manager received new smartcontracts along with their ABI
func (manager *Manager) new_smartcontracts(parameters key_value.KeyValue) {
	manager.logger.Info("add new smartcontracts to the manager")

	raw_smartcontracts, _ := parameters.GetKeyValueList("smartcontracts")
	raw_abis, _ := parameters["abis"].([]interface{})

	new_workers := make(smartcontract.EvmWorkers, len(raw_abis))

	// wait until we receive the new block number
	manager.logger.Info("wait for recent block queue to have atleast one block")
	for {
		if manager.subscribed_blocks.IsEmpty() {
			time.Sleep(time.Second * 1)
			continue
		}
		break
	}

	manager.logger.Info("recent block determined, splitting smartcontracts to old and current")

	for i, raw_abi := range raw_abis {
		sm, _ := categorizer_smartcontract.New(raw_smartcontracts[i])

		abi_data, _ := static_abi.New(raw_abi.(map[string]interface{}))
		cat_abi, err := abi.NewAbi(abi_data)
		if err != nil {
			manager.logger.Fatal("failed to decode", "type", fmt.Sprintf("%T", raw_abi), "index", i, "smartcontract", sm.Address, "errr", err)
		}
		manager.logger.Info("add a new worker", "number", i+1, "total", len(new_workers))
		new_workers[i] = smartcontract.New(sm, cat_abi)
	}

	block_number := manager.subscribed_blocks.First().(*spaghetti_block.Block).BlockNumber

	manager.logger.Info("information about workers", "block_number", block_number, "amount of workers", len(new_workers))

	old_workers, current_workers := new_workers.Sort().Split(block_number)
	old_block_number := old_workers.EarliestBlockNumber()

	manager.logger.Info("splitting to old and new workers", "old amount", len(old_workers), "new amount", len(current_workers))
	manager.logger.Info("old workers information", "earliest_block_number", old_block_number)

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

	manager.logger.Info("add current workers")

	manager.add_current_workers(current_workers)
}

// Categorization of the smartcontracts that are super old.
//
// Get List of smartcontract addresses
// Get Log for the smartcontracts.
func (manager *Manager) categorize_old_smartcontracts(group *OldWorkerGroup) {
	var mu sync.Mutex
	old_logger := app_log.Child(manager.logger, "old_logger_"+time.Now().String())

	url := blockchain_proc.BlockchainManagerUrl(manager.Network.Id)
	blockchain_socket := remote.InprocRequestSocket(url)
	defer blockchain_socket.Close()

	old_logger.Info("starting categorization of old smartcontracts.", "blockchain client manager", url)

	for {
		block_number_from := group.block_number + uint64(1)
		addresses := group.workers.GetSmartcontractAddresses()

		old_logger.Info("fetch from blockchain client manager logs", "block_number", block_number_from, "addresses", addresses)

		all_logs, block_number_to, err := spaghetti_log.RemoteLogFilter(blockchain_socket, block_number_from, addresses)
		if err != nil {
			old_logger.Warn("SKIP, blockchain manager returned an error for block number %d and addresses %v: %w", block_number_from, addresses, err)
			time.Sleep(time.Second)
			continue
		}

		block_timestamp_to := uint64(0)
		if len(all_logs) > 0 {
			block_number_to, block_timestamp_to = spaghetti_log.RecentBlock(all_logs)
		}
		old_logger.Info("fetched from blockchain client manager", "logs amount", len(all_logs), "smartcontract address", addresses, "block_number_to", block_number_to)

		decoded_logs := make([]*categorizer_event.Log, 0)

		// decode the logs
		for _, raw_log := range all_logs {
			for _, worker := range group.workers {
				if worker.Smartcontract.Address != raw_log.Address {
					continue
				}

				decoded_log, err := worker.DecodeLog(raw_log)
				if err != nil {
					old_logger.Error("worker.DecodeLog", "smartcontract", worker.Smartcontract.Address, "message", err)
					continue
				}

				decoded_logs = append(decoded_logs, decoded_log)
			}
		}

		// update the categorization state for the smartcontract
		smartcontracts := group.workers.GetSmartcontracts()
		for _, smartcontract := range smartcontracts {
			new_block_number := block_number_to
			new_block_timestamp := block_timestamp_to

			for _, decoded_log := range decoded_logs {
				if strings.EqualFold(decoded_log.Address, smartcontract.Address) {
					new_block_number = decoded_log.BlockNumber
					new_block_timestamp = decoded_log.BlockTimestamp
				}
			}
			smartcontract.SetBlockParameter(new_block_number, new_block_timestamp)
		}

		// now we send the categorized smartcontracts and logs information
		// to SDS Categorizer, so that SDS Categorizer will update its Database
		push := message.Request{
			Command: "",
			Parameters: map[string]interface{}{
				"smartcontracts": smartcontracts,
				"logs":           decoded_logs,
			},
		}
		request_string, _ := push.ToString()

		mu.Lock()
		_, err = manager.pusher.SendMessage(request_string)
		mu.Unlock()

		if err != nil {
			old_logger.Fatal("send to SDS Categorizer", "message", err)
		}

		recent_block_number := manager.subscribed_blocks.First().(*spaghetti_block.Block).BlockNumber
		left := recent_block_number - block_number_to
		old_logger.Info("categorized certain blocks", "block_number_left", left, "block_number_to", block_number_to, "subscribed", recent_block_number)
		group.block_number = block_number_to

		if block_number_to >= manager.subscribed_blocks.First().(*spaghetti_block.Block).BlockNumber {
			old_logger.Info("catched the current blocks")
			manager.add_current_workers(group.workers)
			break
		}

		// do not pressure the backend
		time.Sleep(time.Second)
	}
	// delete the categorizer group
	manager.old_categorizers = manager.old_categorizers.Delete(group)

	old_logger.Info("finished!")
}

// Move recent to consuming
func (manager *Manager) add_current_workers(workers smartcontract.EvmWorkers) {
	manager.current_workers = append(manager.current_workers, workers...)
}

// Consume each received block from SDS Spaghetti broadcast
func (manager *Manager) categorize_current_smartcontracts() {
	var mu sync.Mutex
	current_logger := app_log.Child(manager.logger, "current")

	current_logger.Info("starting to consume subscribed blocks...")

	for {
		time.Sleep(time.Second * time.Duration(1))

		if len(manager.current_workers) == 0 || manager.subscribed_blocks.IsEmpty() {
			continue
		}

		// consume each block by workers
		for {
			block := manager.subscribed_blocks.Pop().(*spaghetti_block.Block)

			decoded_logs := make([]*categorizer_event.Log, 0)

			// decode the logs
			for _, raw_log := range block.Logs {
				for _, worker := range manager.current_workers {
					if worker.Smartcontract.Address != raw_log.Address {
						continue
					}

					decoded_log, err := worker.DecodeLog(raw_log)
					if err != nil {
						current_logger.Error("worker.DecodeLog", "smartcontract", worker.Smartcontract.Address, "message", err)
						continue
					}

					decoded_logs = append(decoded_logs, decoded_log)
					worker.Smartcontract.SetBlockParameter(decoded_log.BlockNumber, decoded_log.BlockTimestamp)
				}
			}

			push := message.Request{
				Command: "",
				Parameters: map[string]interface{}{
					"smartcontracts": manager.current_workers.GetSmartcontracts(),
					"logs":           decoded_logs,
				},
			}
			request_string, _ := push.ToString()

			current_logger.Info("send a notification to SDS Categorizer")

			mu.Lock()
			_, err := manager.pusher.SendMessage(request_string)

			mu.Unlock()
			if err != nil {
				current_logger.Fatal("sending notification to SDS Categorizer", "message", err)
			}
		}
	}
}

// We start to consume the block information from SDS Spaghetti
// And put it in the queue.
// The worker will start to consume them one by one.
func (manager *Manager) queue_recent_blocks() {
	sub_logger := app_log.Child(manager.logger, "recent_block_queue")

	url := blockchain_proc.BlockchainManagerUrl(manager.Network.Id)
	blockchain_socket := remote.InprocRequestSocket(url)

	request := message.Request{
		Command:    "recent-block-number",
		Parameters: key_value.Empty(),
	}

	recent_reply, err := blockchain_socket.RequestRemoteService(&request)
	if err != nil {
		sub_logger.Fatal("recent-block-number RemoteRequest", "message", err)
	}

	block_number, _ := recent_reply.GetUint64("block_number")
	sub_logger.Info("recent-block-number", "block_number", block_number)
	sub_logger.Info("pause 10 seconds before each log filter")

	for {
		time.Sleep(10 * time.Second)
		if manager.subscribed_blocks.IsFull() {
			sub_logger.Warn("subscribed block is full. Start to consume them [trying in 10 seconds]", "message", err)
			continue
		}

		logs, _, err := spaghetti_log.RemoteLogFilter(blockchain_socket, block_number, []string{})
		if err != nil {
			sub_logger.Warn("failed to get the log filters [trying in 10 seconds]", "message", err)
			continue
		}

		block_number_to, block_timestamp_to := spaghetti_log.RecentBlock(logs)
		sub_logger.Info("recent block number", "block_number", block_number, "block_number of the log", block_number_to, "logs amount", len(logs))

		// we already accumulated the logs
		if block_number_to == block_number {
			continue
		}

		new_block := spaghetti_block.NewBlock(manager.Network.Id, block_number_to, block_timestamp_to, logs)

		sub_logger.Info("add a block to consume", "block_number", block_number_to, "event log amount", len(logs))
		manager.subscribed_blocks.Push(new_block)

		block_number = block_number_to + 1
	}
}
