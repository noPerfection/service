// Spaghetti Worker connects to the blockchain over the loop.
// Worker is running per blockchain network with VM.
package client

import (
	"fmt"
	"time"

	evm_log "github.com/blocklords/sds/blockchain/evm/event"
	"github.com/blocklords/sds/blockchain/handler"
	blockchain_proc "github.com/blocklords/sds/blockchain/inproc"
	"github.com/blocklords/sds/blockchain/network"
	"github.com/blocklords/sds/blockchain/transaction"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote/message"

	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	eth_types "github.com/ethereum/go-ethereum/core/types"

	zmq "github.com/pebbe/zmq4"
)

const (
	ATTEMPT_AMOUNT = 10
	ATTEMPT_DELAY  = time.Duration(time.Second)
)

// The manager of the client.
// Manager encapsulates the client.
// All other services can send data through the manager.
type Manager struct {
	logger     log.Logger
	clients    []*Client
	app_config *configuration.Config
	network    *network.Network
}

// A wrapper around Blockchain Client
// This wrapper sets the connection between blockchain client and SDS.
// All other parts of the SDS interacts with the client through this
func NewManager(network *network.Network, logger log.Logger, app_config *configuration.Config) (*Manager, error) {
	clients, err := new_clients(network.Providers)
	if err != nil {
		return nil, fmt.Errorf("new_clients: %w", err)
	}

	return &Manager{
		clients:    clients,
		logger:     logger,
		network:    network,
		app_config: app_config,
	}, nil
}

// Return the list of clients that has success rating more than 5 percent
func (m *Manager) stable_clients() []*Client {
	clients := make([]*Client, 0)

	for _, c := range m.clients {
		if c.Rating >= STABLE_RATING {
			clients = append(clients, c)
		}
	}

	return clients
}

// Sets up the socket to interact with other packages within SDS
func (worker *Manager) SetupSocket() {
	sock, err := zmq.NewSocket(zmq.REP)
	if err != nil {
		worker.logger.Fatal("trying to create new reply socket for network id %s: %v", worker.network.Id, err)
	}

	url := blockchain_proc.ClientEndpoint(worker.network.Id)
	if err := sock.Bind(url); err != nil {
		worker.logger.Fatal("trying to create categorizer for network id %s: %v", worker.network.Id, err)
	}
	worker.logger.Info("reply controller waiting for messages", "url", url)
	defer sock.Close()

	for {
		// Wait for reply.
		msgs, _ := sock.RecvMessage(0)
		request, _ := message.ParseRequest(msgs)

		var reply message.Reply
		var err error = nil
		if request.Command == handler.FILTER_LOG_COMMAND.String() {
			var request_parameters handler.FilterLog
			err = request.Parameters.ToInterface(&request_parameters)
			if err == nil {
				reply = worker.filter_log(request_parameters)
			}
		} else if request.Command == handler.TRANSACTION_COMMAND.String() {
			var request_parameters handler.Transaction
			err = request.Parameters.ToInterface(&request_parameters)
			if err == nil {
				reply = worker.get_transaction(request_parameters)
			}
		} else if request.Command == handler.TRANSACTION_COMMAND.String() {
			reply = worker.get_recent_block()
		} else {
			reply = message.Fail("unsupported command: " + request.Command)
		}
		if err != nil {
			reply = message.Fail("request parameter: " + err.Error())
		}

		reply_string, err := reply.ToString()
		if err != nil {
			if _, err := sock.SendMessage(err.Error()); err != nil {
				worker.logger.Fatal("reply.ToString error to send message for network id %s error: %w", worker.network.Id, err)
			}
		} else {
			if _, err := sock.SendMessage(reply_string); err != nil {
				worker.logger.Fatal("failed to reply: %w", err)
			}
		}
	}
}

// Handle the filter-log command
// Returns the smartcontract event logs filtered by the smartcontract addresses
func (worker *Manager) filter_log(parameters handler.FilterLog) message.Reply {
	network_id := worker.network.Id
	block_number_from := parameters.BlockFrom.Value()
	addresses := parameters.Addresses

	length, err := worker.network.GetFirstProviderLength()
	if err != nil {
		return message.Fail("failed to get the block range length for first provider of " + network_id)
	}
	block_number_to := block_number_from + length

	raw_logs := make([]eth_types.Log, 0)
	clients := worker.stable_clients()
	if len(clients) == 0 {
		return message.Fail("no stable clients found")
	}
	attempt_failed := 0
	for _, client := range clients {
		var fetched_raw_logs []eth_types.Log

		attempt := ATTEMPT_AMOUNT
		for {
			fetched_raw_logs, err = client.GetBlockRangeLogs(block_number_from, block_number_to, addresses, worker.app_config)
			if err == nil {
				break
			}
			if attempt == 0 {
				attempt_failed++
				break
			}
			time.Sleep(ATTEMPT_DELAY)
			attempt--
		}

		if len(fetched_raw_logs) == 0 {
			continue
		}

		for _, fetched_log := range fetched_raw_logs {
			if fetched_log.Removed {
				continue
			}
			found := false

			for _, raw_log := range raw_logs {
				if raw_log.TxHash.Hex() == fetched_log.TxHash.Hex() &&
					raw_log.Index == fetched_log.Index {
					found = true
					break
				}
			}

			if !found {
				raw_logs = append(raw_logs, fetched_log)
			}
		}
	}
	if attempt_failed == len(clients) {
		return message.Fail("multiple attempts were made : " + err.Error())
	}

	block_number, err := blockchain.NewNumber(block_number_from)
	if err != nil {
		return message.Fail("blockchain.NewNumber: " + err.Error())
	}

	block_timestamp, err := worker.get_block_timestamp(block_number)
	if err != nil {
		return message.Fail("failed to get block timestamp from blockchain: " + err.Error())
	}

	logs, err := evm_log.NewSpaghettiLogs(network_id, block_timestamp, raw_logs)
	if err != nil {
		return message.Fail("evm.NewSpaghettiLogs: " + err.Error())
	}

	reply := message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"logs":     logs,
			"block_to": block_number_to,
		}),
	}

	return reply
}

func (worker *Manager) get_block_timestamp(block_number blockchain.Number) (blockchain.Timestamp, error) {
	clients := worker.stable_clients()
	if len(clients) == 0 {
		return 0, fmt.Errorf("no stable clients found")
	}
	attempt_failed := 0
	var err error
	var block_timestamp uint64 = 0
	for _, client := range clients {
		var fetched_block_timestamp uint64

		attempt := ATTEMPT_AMOUNT
		for {
			fetched_block_timestamp, err = client.GetBlockTimestamp(block_number.Value(), worker.app_config)
			if err == nil {
				break
			}
			if attempt == 0 {
				attempt_failed++
				break
			}
			time.Sleep(ATTEMPT_DELAY)
			attempt--
		}

		if fetched_block_timestamp == 0 {
			worker.logger.Warn("block timestamp is 0", "client", client.provider.Url, "block_number", block_number)
			continue
		}
		if block_timestamp == 0 {
			block_timestamp = fetched_block_timestamp
		} else if block_timestamp != fetched_block_timestamp {
			worker.logger.Warn("the clients returned unmatching block timetstamp", "client", client.provider.Url, "block_number", block_number, "recent_block_timestamp", block_timestamp, "fetched_block_timestamp", fetched_block_timestamp)
			continue
		}
	}
	if attempt_failed == len(clients) {
		return 0, fmt.Errorf("multiple attempts were made")
	}

	new_block_timestamp, err := blockchain.NewTimestamp(block_timestamp)
	if err != nil {
		return 0, fmt.Errorf("blockchain.NewTimestamp: %w", err)
	}

	return new_block_timestamp, nil
}

// Handle the deployed-transaction command
// Returns the transaction information from blockchain
func (worker *Manager) get_transaction(parameters handler.Transaction) message.Reply {
	var tx *transaction.RawTransaction = nil
	var err error
	clients := worker.stable_clients()
	if len(clients) == 0 {
		return message.Fail("no stable clients found")
	}
	attempt_failed := 0
	for _, client := range clients {
		var fetched_tx *transaction.RawTransaction

		attempt := ATTEMPT_AMOUNT
		for {
			fetched_tx, err = client.GetTransaction(parameters.TransactionId, worker.app_config)
			if err == nil {
				break
			}
			if attempt == 0 {
				attempt_failed++
				break
			}
			time.Sleep(ATTEMPT_DELAY)
			attempt--
		}

		if tx == nil {
			tx = fetched_tx
			break
		}
	}
	if attempt_failed == len(clients) {
		return message.Fail("multiple attempts were made : " + err.Error())
	}

	reply := message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"transaction": tx,
		}),
	}

	return reply
}

// Handle the get-recent-block-number command
// Returns the most recent block number and its timestamp
func (worker *Manager) get_recent_block() message.Reply {
	confirmations := uint64(12)

	var block_number uint64 = 0
	var err error
	clients := worker.stable_clients()
	if len(clients) == 0 {
		return message.Fail("no stable clients found")
	}
	attempt_failed := 0
	for _, client := range clients {
		var fetched_block_number uint64

		attempt := ATTEMPT_AMOUNT
		for {
			fetched_block_number, err = client.GetRecentBlockNumber(worker.app_config)
			if err == nil {
				break
			}
			if attempt == 0 {
				attempt_failed++
				break
			}
			time.Sleep(ATTEMPT_DELAY)
			attempt--
		}

		if fetched_block_number == 0 {
			worker.logger.Warn("block timestamp is 0", "client", client.provider.Url)
			continue
		}

		block_number = fetched_block_number
		break
	}
	if attempt_failed == len(clients) {
		return message.Fail("multiple attempts were made : " + err.Error())
	}
	if block_number < confirmations {
		return message.Fail("the recent block number < confirmations")
	}
	block_number -= confirmations
	if block_number == 0 {
		return message.Fail("block number=confirmations")
	}

	recent_block_number, err := blockchain.NewNumber(block_number)
	if err != nil {
		return message.Fail("blockchain.NewNumber: " + err.Error())
	}

	block_timestamp, err := worker.get_block_timestamp(recent_block_number)
	if err != nil {
		return message.Fail("failed to get block timestamp from blockchain: " + err.Error())
	}
	recent_block := blockchain.BlockHeader{
		Number:    recent_block_number,
		Timestamp: block_timestamp,
	}
	recent_block_kv, _ := key_value.NewFromInterface(recent_block)

	reply := message.Reply{
		Status:     "OK",
		Message:    "",
		Parameters: recent_block_kv,
	}

	return reply
}
