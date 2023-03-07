// Spaghetti Worker connects to the blockchain over the loop.
// Worker is running per blockchain network with VM.
package client

import (
	"fmt"
	"time"

	"github.com/charmbracelet/log"

	evm_log "github.com/blocklords/sds/blockchain/evm/event"
	blockchain_proc "github.com/blocklords/sds/blockchain/inproc"
	"github.com/blocklords/sds/blockchain/network"
	"github.com/blocklords/sds/blockchain/transaction"

	"github.com/blocklords/sds/app/remote/message"

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
	logger  log.Logger
	clients []*Client
	network *network.Network
}

// A wrapper around Blockchain Client
// This wrapper sets the connection between blockchain client and SDS.
// All other parts of the SDS interacts with the client through this
func NewManager(network *network.Network, logger log.Logger) (*Manager, error) {
	clients, err := new_clients(network.Providers)
	if err != nil {
		return nil, fmt.Errorf("new_clients: %w", err)
	}

	return &Manager{
		clients: clients,
		logger:  logger,
		network: network,
	}, nil
}

// Returns the client with the best rating
func (m *Manager) client() *Client {
	rating := MIN_RATING_CAP
	index := 0

	for i, c := range m.clients {
		if c.Rating > rating {
			index = i
		}
	}

	return m.clients[index]
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

func (m *Manager) client_info(title string) {
	for i, c := range m.clients {
		m.logger.Info("client info"+title, "id", i, "provider url", c.provider.Url, "rating", c.Rating)
	}
}

// Sets up the socket to interact with other packages within SDS
func (worker *Manager) SetupSocket() {
	sock, err := zmq.NewSocket(zmq.REP)
	if err != nil {
		log.Fatal("trying to create new reply socket for network id %s: %v", worker.network.Id, err)
	}

	url := blockchain_proc.BlockchainManagerUrl(worker.network.Id)
	if err := sock.Bind(url); err != nil {
		log.Fatal("trying to create categorizer for network id %s: %v", worker.network.Id, err)
	}
	worker.logger.Info("reply controller waiting for messages", "url", url)
	defer sock.Close()

	for {
		// Wait for reply.
		msgs, _ := sock.RecvMessage(0)
		request, _ := message.ParseRequest(msgs)

		var reply message.Reply

		if request.Command == "log-filter" {
			reply = worker.filter_log(request.Parameters)
		} else if request.Command == "transaction" {
			reply = worker.get_transaction(request.Parameters)
		} else if request.Command == "recent-block-number" {
			reply = worker.get_recent_block()
		} else {
			reply = message.Fail("unsupported command")
		}

		reply_string, err := reply.ToString()
		if err != nil {
			if _, err := sock.SendMessage(err.Error()); err != nil {
				log.Fatal("reply.ToString error to send message for network id %s error: %w", worker.network.Id, err)
			}
		} else {
			if _, err := sock.SendMessage(reply_string); err != nil {
				log.Fatal("failed to reply: %w", err)
			}
		}
	}
}

// Handle the filter-log command
// Returns the smartcontract event logs filtered by the smartcontract addresses
func (worker *Manager) filter_log(parameters key_value.KeyValue) message.Reply {
	network_id := worker.network.Id
	block_number_from, _ := parameters.GetUint64("block_from")

	addresses, _ := parameters.GetStringList("addresses")

	length, err := worker.network.GetFirstProviderLength()
	if err != nil {
		return message.Fail("failed to get the block range length for first provider of " + network_id)
	}
	block_number_to := block_number_from + length

	attempt := ATTEMPT_AMOUNT
	var raw_logs []eth_types.Log
	for {
		raw_logs, err = worker.client().GetBlockRangeLogs(block_number_from, block_number_to, addresses)
		if err == nil {
			break
		}
		if attempt == 0 {
			return message.Fail("multiple attempts were made unseccsfully: " + err.Error())
		}
		time.Sleep(ATTEMPT_DELAY)
		attempt--
	}

	attempt = ATTEMPT_AMOUNT
	var block_timestamp uint64
	for {
		block_timestamp, err = worker.client().GetBlockTimestamp(block_number_from)
		if err == nil {
			break
		}
		if attempt == 0 {
			return message.Fail("multiple attempts were made unseccsfully: " + err.Error())
		}
		time.Sleep(ATTEMPT_DELAY)
		attempt--
	}

	logs := evm_log.NewSpaghettiLogs(network_id, block_timestamp, raw_logs)

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

// Handle the deployed-transaction command
// Returns the transaction information from blockchain
func (worker *Manager) get_transaction(parameters key_value.KeyValue) message.Reply {
	transaction_id, _ := parameters.GetString("transaction_id")

	var tx *transaction.Transaction
	var err error
	attempt := ATTEMPT_AMOUNT
	for {
		tx, err = worker.client().GetTransaction(transaction_id)
		if err == nil {
			break
		}
		if attempt == 0 {
			return message.Fail("multiple attempts were made unseccsfully: " + err.Error())
		}
		time.Sleep(ATTEMPT_DELAY)
		attempt--
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

	var block_number uint64
	var err error
	attempt := ATTEMPT_AMOUNT
	for {
		block_number, err = worker.client().GetRecentBlockNumber()
		if err == nil {
			break
		}
		if attempt == 0 {
			return message.Fail("multiple attempts were made unseccsfully: " + err.Error())
		}
		time.Sleep(ATTEMPT_DELAY)
		attempt--
	}
	if block_number < confirmations {
		return message.Fail("the recent block number < confirmations")
	}
	block_number -= confirmations
	if block_number == 0 {
		return message.Fail("block number=confirmations")
	}

	var block_timestamp uint64
	attempt = ATTEMPT_AMOUNT
	for {
		block_timestamp, err = worker.client().GetBlockTimestamp(block_number)
		if err == nil {
			break
		}
		if attempt == 0 {
			return message.Fail("multiple attempts were made unseccsfully: " + err.Error())
		}
		time.Sleep(ATTEMPT_DELAY)
		attempt--
	}

	reply := message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"block_number":    block_number,
			"block_timestamp": block_timestamp,
		}),
	}

	return reply
}
