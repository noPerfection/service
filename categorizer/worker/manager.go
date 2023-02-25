// EVM blockchain worker's manager
// For every blockchain we have one manager.
// Manager keeps the list of the smartcontract workers:
// - list of workers for up to date smartcontracts
// - list of workers for categorization outdated smartcontracts
package worker

import (
	"fmt"
	"time"

	"github.com/blocklords/gosds/app/service"
	"github.com/blocklords/gosds/static/network"

	"github.com/blocklords/gosds/categorizer/smartcontract"
	"github.com/blocklords/gosds/common/data_type"

	"github.com/blocklords/gosds/app/argument"
	"github.com/blocklords/gosds/app/remote/message"
	spaghetti_block "github.com/blocklords/gosds/spaghetti/block"
	spaghetti_log "github.com/blocklords/gosds/spaghetti/log"
	zmq "github.com/pebbe/zmq4"

	"github.com/blocklords/gosds/app/remote"
)

const IDLE = "idle"
const RUNNING = "running"

// Manager of the smartcontracts in a particular network
type Manager struct {
	In               chan EvmWorkers
	spaghetti_socket *remote.Socket
	spaghetti_in     chan RequestSpaghettiBlockRange
	spaghetti_out    chan ReplySpaghettiBlockRange
	Network          *network.Network

	old_categorizers OldWorkerGroups

	recent_workers EvmWorkers
	recent_status  string

	current_workers EvmWorkers
	current_status  string

	subscriber_status                string
	subscribed_earliest_block_number uint64
	subscribed_blocks                data_type.Queue
}

// Creates a new manager for the given EVM Network
// New manager runs in the background.
func NewManager(
	network *network.Network,
	in chan RequestSpaghettiBlockRange,
	out chan ReplySpaghettiBlockRange,
) *Manager {
	manager := Manager{
		In:            make(chan EvmWorkers),
		Network:       network,
		spaghetti_in:  in,
		spaghetti_out: out,

		old_categorizers: make(OldWorkerGroups, 0),

		recent_status:  IDLE,
		recent_workers: make(EvmWorkers, 0),

		subscriber_status:                IDLE,
		subscribed_blocks:                *data_type.NewQueue(),
		subscribed_earliest_block_number: 0,

		// consumes the data from the subscribed blocks
		current_status:  IDLE,
		current_workers: make(EvmWorkers, 0),
	}

	return &manager
}

// Returns all smartcontracts from all types of workers
func (manager *Manager) GetSmartcontracts() []*smartcontract.Smartcontract {
	smartcontracts := make([]*smartcontract.Smartcontract, 0)

	for _, group := range manager.old_categorizers {
		smartcontracts = append(smartcontracts, group.workers.GetSmartcontracts()...)
	}

	smartcontracts = append(smartcontracts, manager.recent_workers.GetSmartcontracts()...)
	smartcontracts = append(smartcontracts, manager.current_workers.GetSmartcontracts()...)

	return smartcontracts
}

func (manager *Manager) GetSmartcontractAddresses() []string {
	addresses := make([]string, 0)

	for _, group := range manager.old_categorizers {
		addresses = append(addresses, group.workers.GetSmartcontractAddresses()...)
	}

	addresses = append(addresses, manager.recent_workers.GetSmartcontractAddresses()...)
	addresses = append(addresses, manager.current_workers.GetSmartcontractAddresses()...)

	return addresses
}

// Starts the manager in a background as a goroutine.
// IMPORTANT! it doesn't validate the service configurations
// They should be validated in the main page.
//
// Change the block_get_range to accept multiple addresses
// create a []*Worker data type that manipulates the list of contracts
// - get all below the block number
// - get all above the block number
// - sort from top to bottom
// - update the smartcontract number
// - get list of smartcontract addresses
// - check whether address exists in the list
func (manager *Manager) Run() {
	manager.subscriber_status = RUNNING
	go manager.subscribe()

	manager.current_status = RUNNING
	go manager.categorize_current_smartcontracts()

	categorizer_env, _ := service.New(service.CATEGORIZER, service.BROADCAST, service.THIS)
	spaghetti_env, _ := service.New(service.SPAGHETTI, service.REMOTE)

	manager.spaghetti_socket = remote.TcpRequestSocketOrPanic(spaghetti_env, categorizer_env)

	for {
		new_workers := <-manager.In

		var block_number uint64

		for {
			block_number = manager.subscribed_earliest_block_number
			if block_number == 0 {
				time.Sleep(time.Second * 1)
				continue
			}
			break
		}

		old_workers, current_workers := new_workers.Sort().Split(block_number)
		old_block_number := old_workers.EarliestBlockNumber()

		group := manager.old_categorizers.FirstGroupGreaterThan(old_block_number)
		if group == nil {
			group = NewGroup(old_block_number, old_workers)
			manager.old_categorizers = append(manager.old_categorizers, group)
			go manager.categorize_old_smartcontracts(group)
		} else {
			group.add_workers(old_workers)
		}

		manager.add_current_workers(current_workers)
	}
}

// Categorization of the smartcontracts that are super old.
//
// Get List of smartcontract addresses
// Get Log for the smartcontracts.
func (manager *Manager) categorize_old_smartcontracts(group *OldWorkerGroup) {
	for {
		block_number_from := group.block_number + uint64(1)
		addresses := manager.GetSmartcontractAddresses()

		all_logs, err := spaghetti_log.RemoteLogFilter(manager.spaghetti_socket, manager.Network.Id, block_number_from, addresses)
		if err != nil {
			fmt.Println("failed to get the remote block number for network: " + manager.Network.Id + " error: " + err.Error())
			continue
		}

		// update the worker data by logs.
		block_number_to := block_number_from
		for _, worker := range group.workers {
			logs := spaghetti_log.FilterByAddress(all_logs, worker.smartcontract.Address)
			if len(logs) == 0 {
				continue
			}
			block_number_to, err = worker.categorize(logs)
			if err != nil {
				panic("failed to categorize the blockchain")
			}
		}

		group.block_number = block_number_to

		if block_number_to >= manager.subscribed_earliest_block_number {
			manager.add_current_workers(group.workers)
			break
		}
	}

	// delete the categorizer group
	manager.old_categorizers = manager.old_categorizers.Delete(group)
}

// Move recent to consuming
func (manager *Manager) add_current_workers(workers EvmWorkers) {
	manager.current_workers = append(manager.current_workers, manager.recent_workers...)
}

// Consume each received block from SDS Spaghetti broadcast
func (manager *Manager) categorize_current_smartcontracts() {
	for {
		time.Sleep(time.Second * time.Duration(1))

		if len(manager.current_workers) == 0 || manager.subscribed_blocks.IsEmpty() {
			continue
		}

		// consume each block by workers
		for {
			block := manager.subscribed_blocks.Pop().(*spaghetti_block.Block)

			for _, worker := range manager.current_workers {
				if block.BlockNumber <= worker.smartcontract.CategorizedBlockNumber {
					continue
				}
				logs := block.GetForSmartcontract(worker.smartcontract.Address)
				_, err := worker.categorize(logs)
				if err != nil {
					panic("failed to categorize the blockchain")
				}
			}
		}
	}
}

// We start to consume the block information from SDS Spaghetti
// And put it in the queue.
// The worker will start to consume them one by one.
func (manager *Manager) subscribe() {
	time_out := 20 * time.Second // the longest block mining time among all supported blockchains.

	ctx, err := zmq.NewContext()
	if err != nil {
		panic(err)
	}

	spaghetti_env, _ := service.New(service.SPAGHETTI, service.BROADCAST)
	subscriber, sockErr := ctx.NewSocket(zmq.SUB)
	if sockErr != nil {
		panic(sockErr)
	}

	plain, _ := argument.Exist(argument.PLAIN)

	if !plain {
		categorizer_env, _ := service.New(service.CATEGORIZER, service.SUBSCRIBE)
		subscriber.ClientAuthCurve(spaghetti_env.BroadcastPublicKey, categorizer_env.BroadcastPublicKey, categorizer_env.BroadcastSecretKey)
	}

	conErr := subscriber.Connect("tcp://" + spaghetti_env.BroadcastUrl())
	if conErr != nil {
		panic(conErr)
	}
	err = subscriber.SetSubscribe(manager.Network.Id + " ")
	if err != nil {
		panic(err)
	}

	poller := zmq.NewPoller()
	poller.Add(subscriber, zmq.POLLIN)
	alarm := time.Now().Add(time_out)

	for {
		tickless := time.Until(alarm)
		if tickless < 0 {
			tickless = 0
		}
		polled, err := poller.Poll(tickless)
		if err != nil {
			fmt.Println(manager.Network.Id, "failed to poll SDS Spaghetti Broadcast message", err)
			panic(err)
		}

		if len(polled) == 1 {
			msgRaw, err := subscriber.RecvMessage(0)
			if err != nil {
				fmt.Println(manager.Network.Id, "subscribed message error", err)
				panic(err)
			}

			broadcast, err := message.ParseBroadcast(msgRaw)
			if err != nil {
				fmt.Println(message.Fail("Error when parsing message: " + err.Error()))
				panic(err)
			}

			reply := broadcast.Reply

			block_number, err := reply.Parameters.GetUint64("block_number")
			if err != nil {
				fmt.Println(manager.Network.Id, "error to get the block number", err)
				panic(err)
			}
			network_id, err := reply.Parameters.GetString("network_id")
			if err != nil {
				fmt.Println(manager.Network.Id, "failed to get the network_id from the reply params")
				panic(err)
			}
			if network_id != manager.Network.Id {
				fmt.Println(manager.Network.Id, `skipping unsupported network. it should not be as is`)
				continue
			}

			// Repeated subscriptions are not catched
			if manager.subscribed_earliest_block_number != 0 && block_number < manager.subscribed_earliest_block_number {
				continue
			} else if manager.subscribed_earliest_block_number == 0 {
				manager.subscribed_earliest_block_number = block_number
			}

			timestamp, err := reply.Parameters.GetUint64("block_timestamp")
			if err != nil {
				fmt.Printf(manager.Network.Id, "error getting block timestamp", err)
				panic(err)
			}

			raw_logs, ok := reply.Parameters.ToMap()["logs"].([]interface{})
			if !ok {
				fmt.Println(manager.Network.Id, "failed to get logs from SDS Spaghetti Broadcast")
				panic("no logs received from SDS Spaghetti Broadcast")
			}
			logs, err := spaghetti_log.NewLogs(raw_logs)
			if err != nil {
				fmt.Println(raw_logs...)
				fmt.Println(manager.Network.Id, "failed to parse log", err)
				panic(err)
			}

			new_block := spaghetti_block.NewBlock(manager.Network.Id, block_number, timestamp, logs)

			manager.subscribed_blocks.Push(new_block)
		}

		alarm = time.Now().Add(time_out)
	}
}
