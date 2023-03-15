package worker

import (
	"errors"
	"time"

	"github.com/blocklords/sds/blockchain/imx"
	blockchain_proc "github.com/blocklords/sds/blockchain/inproc"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote/message"
	spaghetti_log "github.com/blocklords/sds/blockchain/event"
	"github.com/blocklords/sds/blockchain/imx/client"
	"github.com/blocklords/sds/common/data_type/key_value"

	zmq "github.com/pebbe/zmq4"
)

// the global variables that we pass between functions in this worker.
// the functions are recursive.
type Manager struct {
	client             *client.Client
	logger             log.Logger
	request_per_second uint64
	request_amount     uint64 // concurrent running requests
}

// A new Manager
func New(app_config *configuration.Config, client *client.Client, logger log.Logger) *Manager {
	return &Manager{
		client:             client,
		logger:             logger,
		request_per_second: app_config.GetUint64(imx.REQUEST_PER_SECOND),
		request_amount:     0,
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
		worker.logger.Fatal("trying to create categorizer for network id %s: %v", worker.client.Network.Id, err)
	}

	worker.logger.Info("reply controller waiting for messages", "url", url)

	for {
		// Wait for reply.
		msgs, _ := sock.RecvMessage(0)
		request, _ := message.ParseRequest(msgs)

		var reply message.Reply

		if request.Command == "log-filter" {
			reply = worker.filter_log(request.Parameters)
		} else {
			reply = message.Fail("unsupported command " + request.Command)
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
	addresses, _ := parameters.GetStringList("addresses")
	address := addresses[0]

	block_timestamp, _ := parameters.GetUint64("block_from")
	timestamp := time.Unix(int64(block_timestamp), 0).UTC().Format(time.RFC3339)

	block_timestamp_to := uint64(block_timestamp)
	timestamp_to := time.Unix(int64(block_timestamp_to), 0).UTC().Format(time.RFC3339)

	worker.request_amount++
	delay_duration := worker.delay_duration()
	transfers, err := worker.client.GetSmartcontractTransferLogs(address, delay_duration, timestamp, timestamp_to)
	if err != nil {
		worker.request_amount--
		return message.Fail("client.GetSmartcontractTransferLogs: " + err.Error())
	}
	if len(transfers) > 0 {
		recent_block := spaghetti_log.RecentBlock(transfers)
		block_timestamp_to = recent_block.Timestamp.Value()

		timestamp_to = time.Unix(int64(block_timestamp_to), 0).UTC().Format(time.RFC3339)
	}

	worker.request_amount++
	delay_duration = worker.delay_duration()
	mints, err := worker.client.GetSmartcontractMintLogs(address, delay_duration, timestamp, timestamp_to)
	if err != nil {
		worker.request_amount--
		return message.Fail("client.GetSmartcontractMingLogs: " + err.Error())
	}

	transfers = append(transfers, mints...)

	if len(transfers) > 0 {
		recent_block := spaghetti_log.RecentBlock(transfers)
		block_timestamp_to = recent_block.Timestamp.Value()
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

// Based on total amount of smartcontracts, how long we delay to request to ImmutableX nodes
func (manager *Manager) delay_duration() time.Duration {
	per_second := float64(manager.request_per_second)
	amount := float64(manager.request_amount)

	return time.Duration(float64(time.Millisecond) * amount * 1000 / per_second)
}
