package worker

import (
	"errors"
	"log"
	"time"

	blockchain_proc "github.com/blocklords/gosds/blockchain/inproc"

	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/blockchain/imx/client"
	"github.com/blocklords/gosds/common/data_type/key_value"

	zmq "github.com/pebbe/zmq4"
)

// the global variables that we pass between functions in this worker.
// the functions are recursive.
type SpaghettiWorker struct {
	client            *client.Client
	broadcast_channel chan message.Broadcast
	debug             bool
}

// A new SpaghettiWorker
func New(client *client.Client, broadcast_channel chan message.Broadcast, debug bool) *SpaghettiWorker {
	return &SpaghettiWorker{
		client:            client,
		broadcast_channel: broadcast_channel,
		debug:             debug,
	}
}

// Sets up the socket to interact with the clients
func (worker *SpaghettiWorker) SetupSocket() {
	sock, err := zmq.NewSocket(zmq.REP)
	if err != nil {
		panic(err)
	}

	url := blockchain_proc.BlockchainManagerUrl(worker.client.Network.Id)
	if err := sock.Bind("inproc://" + url); err != nil {
		log.Fatalf("trying to create categorizer for network id %s: %v", worker.client.Network.Id, err)
	}

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

func (worker *SpaghettiWorker) filter_log(parameters key_value.KeyValue) message.Reply {
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

	reply := message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"logs": transfers,
		}),
	}

	return reply
}

func (worker *SpaghettiWorker) get_transaction(_ key_value.KeyValue) message.Reply {
	return message.Fail("get-transaction is not supported by imx network")
}
