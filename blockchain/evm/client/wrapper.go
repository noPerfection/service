// Spaghetti Worker connects to the blockchain over the loop.
// Worker is running per blockchain network with VM.
package client

import (
	"time"

	app_log "github.com/blocklords/gosds/app/log"
	"github.com/blocklords/gosds/app/service"
	"github.com/charmbracelet/log"

	evm_log "github.com/blocklords/gosds/blockchain/evm/event"
	blockchain_proc "github.com/blocklords/gosds/blockchain/inproc"

	"github.com/blocklords/gosds/app/remote/message"

	"github.com/blocklords/gosds/common/data_type/key_value"

	zmq "github.com/pebbe/zmq4"
)

// the global variables that we pass between functions in this worker.
// the functions are recursive.
type SpaghettiWorker struct {
	logger log.Logger
	client *Client
}

// A wrapper around Blockchain Client
// This wrapper sets the connection between blockchain client and SDS.
// All other parts of the SDS interacts with the client through this
func NewWrapper(client *Client, logger log.Logger) *SpaghettiWorker {
	return &SpaghettiWorker{
		client: client,
		logger: logger,
	}
}

// Sets up the socket to interact with other packages within SDS
func (worker *SpaghettiWorker) SetupSocket() {
	sock, err := zmq.NewSocket(zmq.REP)
	if err != nil {
		log.Fatal("trying to create new reply socket for network id %s: %v", worker.client.Network.Id, err)
	}

	url := blockchain_proc.BlockchainManagerUrl(worker.client.Network.Id)
	if err := sock.Bind(url); err != nil {
		log.Fatal("trying to create categorizer for network id %s: %v", worker.client.Network.Id, err)
	}
	worker.logger.Info("reply controller waiting for messages", "url", url)

	for {
		// Wait for reply.
		msgs, _ := sock.RecvMessage(0)
		request, _ := message.ParseRequest(msgs)

		worker.logger.Info("received a message", "command", request.Command)
		worker.logger.Fatal("remove it")

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

		worker.logger.Info("command handled", "reply_status", reply.Status)

		reply_string, err := reply.ToString()
		if err != nil {
			if _, err := sock.SendMessage(err.Error()); err != nil {
				log.Fatal("reply.ToString error to send message for network id %s error: %w", worker.client.Network.Id, err)
			}
		} else {
			if _, err := sock.SendMessage(reply_string); err != nil {
				log.Fatal("failed to reply: %w", err)
			}
		}
	}
}

func (worker *SpaghettiWorker) filter_log(parameters key_value.KeyValue) message.Reply {
	network_id := worker.client.Network.Id
	block_number_from, _ := parameters.GetUint64("block_from")

	addresses, _ := parameters.GetStringList("addresses")

	length, err := worker.client.Network.GetFirstProviderLength()
	if err != nil {
		return message.Fail("failed to get the block range length for first provider of " + network_id)
	}
	block_number_to := block_number_from + length

	raw_logs, err := worker.client.GetBlockRangeLogs(block_number_from, block_number_to, addresses)
	if err != nil {
		return message.Fail("client.GetBlockRangeLogs: " + err.Error())
	}

	block_timestamp, err := worker.client.GetBlockTimestamp(block_number_from)
	if err != nil {
		return message.Fail("client.GetBlockTimestamp: " + err.Error())
	}

	logs := evm_log.NewSpaghettiLogs(network_id, block_timestamp, raw_logs)

	reply := message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"logs": logs,
		}),
	}

	return reply
}

func (worker *SpaghettiWorker) get_transaction(parameters key_value.KeyValue) message.Reply {
	transaction_id, _ := parameters.GetString("transaction_id")

	tx, err := worker.client.GetTransaction(transaction_id)
	if err != nil {
		return message.Fail("failed to get the block range length for first provider of " + worker.client.Network.Id)
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

func (worker *SpaghettiWorker) get_recent_block() message.Reply {
	confirmations := uint64(12)

	var block_number uint64
	var err error
	for {
		block_number, err = worker.client.GetRecentBlockNumber()
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		break
	}
	if block_number < confirmations {
		return message.Fail("the recent block number < confirmations")
	}
	block_number -= confirmations
	if block_number == 0 {
		return message.Fail("block number=confirmations")
	}

	var block_timestamp uint64
	for {
		block_timestamp, err = worker.client.GetBlockTimestamp(block_number)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		break
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

// Any EVM blockchain doesn't have an indexer.
// Therefore we make an indexer to track the most recent block logs
func (worker *SpaghettiWorker) Sync() {
	sync_logger := app_log.Child(worker.logger, "sync")

	broadcast_pusher, _ := blockchain_proc.NewBroadcastPusher(service.SPAGHETTI.ToString())

	sync_logger.Info("get recent block number from client")

	confirmations := uint64(12)

	var block_number uint64
	var err error
	for {
		block_number, err = worker.client.GetRecentBlockNumber()
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		break
	}
	if block_number < confirmations {
		sync_logger.Fatal("the recent block number > confirmation ", "block_number", block_number, "confirmations", confirmations)
	}
	block_number -= confirmations
	if block_number == 0 {
		sync_logger.Fatal("block number is equal to confirmations. should be block number > confirmations. ", "block_number", block_number)
	}

	sync_logger.Info("the most recent block number", "block_number", block_number)

	// optimize in case of the error
	// or slow internet connection
	// we need to get the data as fast as possible
	//
	// optimization could mean, we use worker.client.GetBlockRangeLogs
	for {
		timestamp, err := worker.client.GetBlockTimestamp(block_number)
		if err != nil {
			sync_logger.Warn(`client.GetBlockTimestamp, retreiving in 10 second`, "block_number", block_number, "message", err)
			time.Sleep(10 * time.Second)
			continue
		}
		raw_logs, err := worker.client.GetBlockLogs(block_number)
		if err != nil {
			sync_logger.Warn(`client.GetBlockLogs, retreiving in 10 second`, "block_number", block_number, "message", err)
			time.Sleep(10 * time.Second)
			continue
		}
		logs := evm_log.NewSpaghettiLogs(worker.client.Network.Id, timestamp, raw_logs)

		new_reply := message.Reply{
			Status:  "OK",
			Message: "",
			Parameters: map[string]interface{}{
				"network_id":      worker.client.Network.Id,
				"block_number":    block_number,
				"block_timestamp": timestamp,
				"logs":            logs,
			},
		}

		sync_logger.Info("send to broadcaster about new block", "topic", worker.client.Network.Id, "block_number", block_number, "logs_amount", len(logs))

		broadcast := message.NewBroadcast(worker.client.Network.Id+" ", new_reply)
		broadcast_string := string(broadcast.ToBytes())

		_, err = broadcast_pusher.SendMessage(broadcast_string)
		if err != nil {
			sync_logger.Fatal("failed to push to broadcaster", "message", err)
		}

		time.Sleep(1 * time.Second)

		block_number++
	}
}
