// The sdk/subscriber package is used to register for the smartcontracts
package subscriber

import (
	"errors"
	"fmt"
	"time"

	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/app/service"
	"github.com/blocklords/gosds/categorizer"
	"github.com/blocklords/gosds/common/data_type"
	"github.com/blocklords/gosds/static"

	"github.com/blocklords/gosds/sdk/db"
)

type Subscriber struct {
	Address           string // Account address granted for reading
	socket            *remote.Socket
	db                *db.KVM                    // it also keeps the topic filter
	smartcontractKeys []*static.SmartcontractKey // list of smartcontract keys

	BroadcastChan   chan message.Broadcast
	broadcastSocket *remote.Socket
}

// Create a new subscriber for a given user and his topic filter.
func NewSubscriber(gatewaySocket *remote.Socket, db *db.KVM, address string, clear_cache bool) (*Subscriber, error) {
	subscriber := Subscriber{
		Address:           address,
		socket:            gatewaySocket,
		db:                db,
		smartcontractKeys: make([]*static.SmartcontractKey, 0),
	}

	err := subscriber.load_smartcontracts(clear_cache)
	if err != nil {
		return nil, err
	}

	return &subscriber, nil
}

// Connect the client to the SDS Publisher broadcast.
// Then start to queue the incoming data from the broadcaster.
// The queued messages will be read and cached by the Subscriber.read_from_publisher() after getting the snapshot.
func (subscriber *Subscriber) connect_to_publisher() error {
	gateway_env, err := service.New(service.DEVELOPER_GATEWAY, service.SUBSCRIBE)
	if err != nil {
		return err
	}
	developer_env := service.NewDeveloper()

	// Run the Subscriber that is connected to the Broadcaster
	subscriber.broadcastSocket = remote.TcpSubscriberOrPanic(gateway_env, developer_env)

	// Subscribing to the events, but we will not call the sub.ReceiveMessage
	// until we will not get the snapshot of the missing data.
	// ZMQ will queue the data until we will not call sub.ReceiveMessage.
	for _, key := range subscriber.smartcontractKeys {
		err := subscriber.broadcastSocket.SetSubscribeFilter(string(*key))
		if err != nil {
			subscriber.broadcastSocket.Close()
			return fmt.Errorf("failed to subscribe to the smartcontract: " + err.Error())
		}
	}

	return nil
}

// The Start() method creates a channel for sending the data to the client.
// Then it connects to the SDS Gateway to get the snapshots.
// Finally, it will receive the messages from SDS Publisher.
func (s *Subscriber) Start() error {
	fmt.Println("Starting the subscription!")

	if err := s.connect_to_publisher(); err != nil {
		return err
	}

	fmt.Println("Subscriber connected and queueing the messages while snapshot won't be ready")

	// now create a broadcaster channel to send back to the developer the messages
	s.BroadcastChan = make(chan message.Broadcast)

	go s.get_data()
	return nil
}

// Returns the latest updated block timestamp in the cache
func (s *Subscriber) recent_block_timestamp() uint64 {
	var recent_block_timestamp uint64 = 0
	for _, key := range s.smartcontractKeys {
		block_timestamp := s.db.GetBlockTimestamp(*key)
		fmt.Println("recent block timestamp: ", *key, block_timestamp)
		if block_timestamp > recent_block_timestamp {
			recent_block_timestamp = block_timestamp
		}
	}

	return recent_block_timestamp
}

// Get the snapshot since the latest cached till the most recent updated time.
func (s *Subscriber) get_snapshot() error {
	limit := uint64(500)
	page := uint64(1)
	block_timestamp_from := s.recent_block_timestamp()
	// if block_timestamp_to is 0, then get snapshot till the most recent block update.
	block_timestamp_to := uint64(0)

	for {
		request := message.Request{
			Command: "snapshot_get",
			Parameters: map[string]interface{}{
				"smartcontract_keys":   data_type.ToStringList(s.smartcontractKeys),
				"block_timestamp_from": block_timestamp_from,
				"block_timestamp_to":   block_timestamp_to,
				"page":                 page,
				"limit":                limit,
			},
		}

		snapshot_parameters, err := s.socket.RequestRemoteService(&request)
		if err != nil {
			return err
		}

		raw_transactions, err := message.GetMapList(snapshot_parameters, "transactions")
		if err != nil {
			return err
		}
		raw_logs, err := message.GetMapList(snapshot_parameters, "logs")
		if err != nil {
			return err
		}
		timestamp, err := message.GetUint64(snapshot_parameters, "block_timestamp")
		if err != nil {
			return err
		}

		// we fetch until all is not received
		if len(raw_transactions) == 0 {
			return nil
		}

		transactions := make([]*categorizer.Transaction, len(raw_transactions))
		logs := make([]*categorizer.Log, len(raw_logs))

		// Saving the latest block number in the cache
		// along the parsing raw data into SDS data type
		for i, rawTx := range raw_transactions {
			tx, err := categorizer.ParseTransaction(rawTx)
			if err != nil {
				return errors.New("failed to parse the transaction. the error: " + err.Error())
			} else {
				transactions[i] = tx
			}

			key := static.CreateSmartcontractKey(tx.NetworkId, tx.Address)
			cached_block_timestamp := s.db.GetBlockTimestamp(key)
			if tx.BlockTimestamp > cached_block_timestamp {
				err = s.db.SetBlockTimestamp(key, tx.BlockTimestamp)
				if err != nil {
					return err
				}
			}
		}
		for i, raw_log := range raw_logs {
			log, err := categorizer.ParseLog(raw_log)
			if err != nil {
				return errors.New("failed to parse the log. the error: " + err.Error())
			}
			logs[i] = log
		}

		reply := message.Reply{
			Status:  "OK",
			Message: "",
			Params: map[string]interface{}{
				"transactions":    transactions,
				"logs":            logs,
				"block_timestamp": timestamp,
			},
		}
		s.BroadcastChan <- message.NewBroadcast("OK", reply)

		if block_timestamp_to == 0 {
			block_timestamp_to = timestamp
		}
		page++

	}
}

// calls the snapshot then incoming data in real-time from SDS Publisher
func (s *Subscriber) get_data() {
	err := s.get_snapshot()
	if err != nil {
		s.BroadcastChan <- message.NewBroadcast("error", message.Fail(err.Error()))
		return
	}

	err = s.read_from_publisher()
	if err != nil {
		s.BroadcastChan <- message.NewBroadcast("error", message.Fail(err.Error()))
	}
}

// Get the list of the smartcontracts by smartcontract filter from SDS Categorizer via SDS Gateway
// Then cache them out and list in the Subscriber data structure
func (s *Subscriber) load_smartcontracts(clear_cache bool) error {
	// preparing the subscriber so that we catch the first message if it was send
	// by publisher.

	smartcontracts, topicStrings, err := static.RemoteSmartcontracts(s.socket, s.db.TopicFilter())
	if err != nil {
		return err
	}

	// set the smartcontract keys
	for i, sm := range smartcontracts {
		key := sm.KeyString()

		if clear_cache {
			err := s.db.DeleteBlockTimestamp(key)
			if err != nil {
				return err
			}
		}
		// cache the smartcontract block timestamp
		// block timestamp is used to subscribe for the events
		blockTimestamp := s.db.GetBlockTimestamp(key)

		if blockTimestamp == 0 {
			blockTimestamp = uint64(sm.PreDeployBlockTimestamp)
			err := s.db.SetBlockTimestamp(key, blockTimestamp)
			if err != nil {
				return err
			}
		}

		// cache the topic string
		topicString := topicStrings[i]
		err = s.db.SetTopicString(key, topicString)
		if err != nil {
			return err
		}

		// finally track the smartcontract
		s.smartcontractKeys = append(s.smartcontractKeys, &key)
	}

	return nil
}

func (s *Subscriber) close(exit_channel chan int) error {
	// Close the previous channel
	exit_channel <- 1

	return s.broadcastSocket.Close()
}

// In case of the failure to read the data from the Publisher
// Or there might be a delay.
// What we do is to reconnect the client to the SDS.
// Get the snapshot of the missing data, then reconnect the subscriber to read data from SDS Publisher.
func (s *Subscriber) reconnect(receive_channel chan message.Reply, exit_channel chan int, time_out time.Duration) error {
	// Close the previous channel
	exit_channel <- 1

	err := s.broadcastSocket.Close()
	if err != nil {
		return err
	}
	fmt.Println("now restarting the subscriber")

	if err := s.connect_to_publisher(); err != nil {
		s.BroadcastChan <- message.NewBroadcast("error", message.Fail("failed to connect to the publisher: "+err.Error()))
		return err
	}

	// get the data that appeared on the SDS Side during the timeout.
	if err := s.get_snapshot(); err != nil {
		s.BroadcastChan <- message.NewBroadcast("error", message.Fail(err.Error()))
		close_err := s.broadcastSocket.Close()
		if close_err != nil {
			return close_err
		}
		return err
	}

	go s.broadcastSocket.Subscribe(receive_channel, exit_channel, time_out)

	return nil
}

// Calls the gosds/remote.Subscriber.Subscribe(),
// Then sends the message to the user.
//
// Returns gosds/message.Reply as a failure or a success.
//
// If the messages are received successfully from the blockchain, then
// gosds/message.Reply.Params will contain the following parameter:
//
//		Reply.Params: {
//			"topic_string": gosds/topic.Topic.ToString(),		// the smartcontract topic string
//			"block_timestamp": uint64,							// the latest block timestmap
//	         "transactions": []gosds/categorizer.Transaction,	// transactions
//	         "logs": []gosds/categorizer.Log,					// smartcontract events
//		}
func (s *Subscriber) read_from_publisher() error {
	receive_channel := make(chan message.Reply)
	exit_channel := make(chan int)
	time_out := time.Duration(time.Second * 30)

	go s.broadcastSocket.Subscribe(receive_channel, exit_channel, time_out)

	for {
		reply := <-receive_channel

		if !reply.IsOK() {
			if reply.Message == "timeout" {
				err := s.reconnect(receive_channel, exit_channel, time_out)
				if err != nil {
					return err
				}
				// wait for another incoming messages
				continue
			} else {
				if err := s.close(exit_channel); err != nil {
					return err
				}
				received_err := errors.New("received an error from subscription: " + reply.Message)
				return received_err
			}
		}

		// validate the parameters
		networkId, err := message.GetString(reply.Params, "network_id")
		if err != nil {
			if close_err := s.close(exit_channel); close_err != nil {
				return errors.New("the sds publisher invalid 'network_id'. failed to close the subscriber loop. error " + close_err.Error())
			}
			return errors.New("the sds publisher invalid 'network_id'. reconnect and try again until publisher won't fix it. error " + err.Error())
		}
		address, err := message.GetString(reply.Params, "address")
		if err != nil {
			if close_err := s.close(exit_channel); close_err != nil {
				return errors.New("the sds publisher invalid 'address'. failed to close the subscriber loop. error " + close_err.Error())
			}
			return errors.New("the sds publisher invalid 'address'. reconnect and try again until publisher won't fix it. error " + err.Error())
		}
		block_timestamp, err := message.GetUint64(reply.Params, "block_timestamp")
		if err != nil {
			if close_err := s.close(exit_channel); close_err != nil {
				return errors.New("the sds publisher invalid 'block_timestamp'. failed to close the subscriber loop. error " + close_err.Error())
			}
			return errors.New("the sds publisher invalid 'block_timestamp'. reconnect and try again until publisher won't fix it. error " + err.Error())
		}

		// Return the data to the SDK client.
		// The SDK returns already formatted data instead of the generic interfaces.

		// receive the transactions and logs of the smartcontract
		raw_transactions, err := message.GetMapList(reply.Params, "transactions")
		if err != nil {
			if close_err := s.close(exit_channel); close_err != nil {
				return errors.New("the sds publisher invalid 'transactions'. failed to close the subscriber loop. error " + close_err.Error())
			}
			return errors.New("the sds publisher invalid 'transactions'. reconnect and try again until publisher won't fix it. error " + err.Error())
		}
		raw_logs, err := message.GetMapList(reply.Params, "logs")
		if err != nil {
			if close_err := s.close(exit_channel); close_err != nil {
				return errors.New("the sds publisher invalid 'logs'. failed to close the subscriber loop. error " + close_err.Error())
			}
			return errors.New("the sds publisher invalid 'logs'. reconnect and try again until publisher won't fix it. error " + err.Error())
		}

		key := static.CreateSmartcontractKey(networkId, address)

		// we skip the duplicate messages that were fetched by the Snapshot
		if s.db.GetBlockTimestamp(key) > block_timestamp {
			continue
		}

		transactions := make([]*categorizer.Transaction, len(raw_transactions))
		for i, raw := range raw_transactions {
			transaction, err := categorizer.ParseTransaction(raw)
			if err != nil {
				if close_err := s.close(exit_channel); close_err != nil {
					return errors.New("failed to parse the transaction " + err.Error() + ", . failed to close the subscriber loop. error " + close_err.Error())
				}
				return errors.New("the sds publisher invalid 'transactions'. failed to parse it. error " + err.Error())
			}

			transactions[i] = transaction
		}

		logs := make([]*categorizer.Log, len(raw_logs))
		for i, raw := range raw_logs {
			log, err := categorizer.ParseLog(raw)
			if err != nil {
				if close_err := s.close(exit_channel); close_err != nil {
					return errors.New("failed to parse the log " + err.Error() + ", . failed to close the subscriber loop. error " + close_err.Error())
				}
				return errors.New("the sds publisher invalid 'logs'. failed to parse it. error " + err.Error())
			}

			logs[i] = log
		}

		// Update the timestamp in the cache only if the received data is valid.
		err = s.db.SetBlockTimestamp(key, block_timestamp)
		if err != nil {
			if close_err := s.close(exit_channel); close_err != nil {
				return errors.New("failed to update the local cache: " + err.Error() + ", . failed to close the subscriber loop. error " + close_err.Error())
			}
			return errors.New("the local cache saving error " + err.Error())
		}

		return_reply := message.Reply{
			Status:  "OK",
			Message: "",
			Params: map[string]interface{}{
				"topic_string":    s.db.GetTopicString(key),
				"block_timestamp": block_timestamp,
				"transactions":    transactions,
				"logs":            logs,
			},
		}

		s.BroadcastChan <- message.NewBroadcast("OK", return_reply)
	}
}
