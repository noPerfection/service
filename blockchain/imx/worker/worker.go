package worker

import (
	"errors"
	"time"

	"github.com/charmbracelet/log"

	blockchain_proc "github.com/blocklords/gosds/blockchain/inproc"

	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/blockchain/imx/client"
	"github.com/blocklords/gosds/common/data_type/key_value"

	zmq "github.com/pebbe/zmq4"
)

// the global variables that we pass between functions in this worker.
// the functions are recursive.
type Manager struct {
	client *client.Client
	logger log.Logger
}

// A new Manager
func New(client *client.Client, logger log.Logger) *Manager {
	return &Manager{
		client: client,
		logger: logger,
	}
}

// Sets up the socket to interact with the clients
func (worker *Manager) SetupSocket() {
	sock, err := zmq.NewSocket(zmq.REP)
	if err != nil {
		panic(err)
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

		var reply message.Reply

		if request.Command == "log-filter" {
			reply = worker.filter_log(request.Parameters)
		}

		reply_string, err := reply.ToString()
		if err != nil {
			if _, err := sock.SendMessage(err.Error()); err != nil {
				panic(err)
			}
		} else {
			if _, err := sock.SendMessage(reply_string); err != nil {
				panic(errors.New("failed to reply: %w" + err.Error()))
			}
		}
	}
}

func (worker *Manager) filter_log(parameters key_value.KeyValue) message.Reply {
	block_timestamp, _ := parameters.GetUint64("block_from")
	timestamp := time.Unix(int64(block_timestamp), 0).Format(time.RFC3339)

	addresses, _ := parameters.GetStringList("addresses")
	address := addresses[0]

	// todo
	// when the categorizer.manager.delay_per_second should be moved to here
	transfers, err := worker.client.GetSmartcontractTransferLogs(10, address, time.Duration(time.Second*1), timestamp)
	if err != nil {
		return message.Fail("client.GetSmartcontractTransferLogs: " + err.Error())
	}

	mints, err := worker.client.GetSmartcontractMintLogs(10, address, time.Duration(time.Second*1), timestamp)
	if err != nil {
		return message.Fail("client.GetSmartcontractMingLogs: " + err.Error())
	}

	transfers = append(transfers, mints...)

	block_timestamp_to := block_timestamp
	if len(transfers) > 0 {
		for _, t := range transfers {
			if t.BlockTimestamp > block_timestamp_to {
				block_timestamp_to = t.BlockTimestamp
			}
		}
	}

	reply := message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"logs":     transfers,
			"block_to": block_timestamp_to,
		}),
	}

	return reply
}

func (worker *Manager) get_transaction(_ key_value.KeyValue) message.Reply {
	return message.Fail("get-transaction is not supported by imx network")
}
