// The subscriber package pushes to the SDK user the smartcontract event logs
// From SDS Categorizer.
//
// How it works:
//
//		We call the get_snapshot() command from the gateway.
//		We call it every one second.
//	 	At the beginning we first verify the topic filter.
package subscriber

import (
	"fmt"
	"time"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/categorizer/event"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/common/topic"
)

type Subscriber struct {
	topic_filter *topic.TopicFilter
	developer    *service.Service
	gateway      *service.Service

	BroadcastChan chan message.Broadcast
}

// Create a new subscriber for a given user and his topic filter.
func NewSubscriber(topic_filter *topic.TopicFilter, developer *service.Service, gateway *service.Service) (*Subscriber, error) {
	subscriber := Subscriber{
		topic_filter: topic_filter,
		developer:    developer,
		gateway:      gateway,
	}

	return &subscriber, nil
}

// The Start() method creates a channel for sending the data to the client.
// Then it connects to the SDS Gateway to get the snapshots.
// Finally, it will receive the messages from SDS Publisher.
func (s *Subscriber) Start() error {
	// now create a broadcaster channel to send back to the developer the messages
	s.BroadcastChan = make(chan message.Broadcast)

	go s.start()
	return nil
}

// Get the snapshot since the latest cached till the most recent updated time.
func (s *Subscriber) get_snapshot(socket *remote.Socket, block_timestamp_from uint64) (uint64, []*event.Log, error) {
	request := message.Request{
		Command: "snapshot_get",
		Parameters: map[string]interface{}{
			"topic_filter":         s.topic_filter,
			"block_timestamp_from": block_timestamp_from,
		},
	}

	snapshot_parameters, err := socket.RequestRemoteService(&request)
	if err != nil {
		return 0, nil, fmt.Errorf("remote snapshot_gets: %w", err)
	}

	raw_logs, err := snapshot_parameters.GetKeyValueList("logs")
	if err != nil {
		return 0, nil, fmt.Errorf("remote snapshot logs parameter: %w", err)
	}
	block_timestamp, err := snapshot_parameters.GetUint64("block_timestamp")
	if err != nil {
		return 0, nil, fmt.Errorf("remote snapshot block timestamp parameter: %w", err)
	}

	logs := make([]*event.Log, len(raw_logs))

	// Saving the latest block number in the cache
	// along the parsing raw data into SDS data type
	for i, raw_log := range raw_logs {
		log, err := event.NewFromMap(raw_log)
		if err != nil {
			return 0, nil, fmt.Errorf("parsing remote snapshot log: %w", err)
		}
		logs[i] = log
	}

	return block_timestamp, logs, nil
}

// calls the snapshot then incoming data in real-time from SDS Publisher
func (s *Subscriber) start() {
	socket := remote.TcpRequestSocketOrPanic(s.gateway, s.developer)

	block_timestamp, err := s.recent_subscriber_state(socket)
	if err != nil {
		s.BroadcastChan <- message.NewBroadcast("error", message.Fail("recent_subscriber_state: "+err.Error()))
	}

	fmt.Println("Subscriber connected and queueing the messages while snapshot won't be ready")

	for {
		block_timestamp_to, logs, err := s.get_snapshot(socket, block_timestamp)
		if err != nil {
			s.BroadcastChan <- message.NewBroadcast("error", message.Fail("snapshot error: "+err.Error()))
			return
		}

		reply := message.Reply{
			Status:  "OK",
			Message: "",
			Parameters: map[string]interface{}{
				"logs":                 logs,
				"block_timestamp_from": block_timestamp,
				"block_timestamp_to":   block_timestamp_to,
			},
		}
		s.BroadcastChan <- message.NewBroadcast("OK", reply)

		block_timestamp = block_timestamp_to

		time.Sleep(time.Second)
	}

}

// Get the recent logs timestamp from where we should continue to fetch
func (s *Subscriber) recent_subscriber_state(socket *remote.Socket) (uint64, error) {
	request := message.Request{
		Command:    "subscriber_state",
		Parameters: key_value.Empty().Set("topic_filter", s.topic_filter),
	}

	parameters, err := socket.RequestRemoteService(&request)
	if err != nil {
		return 0, fmt.Errorf("remote subsriber_state: %w", err)
	}

	block_timestamp, err := parameters.GetUint64("block_timestamp")
	if err != nil {
		return 0, fmt.Errorf("get block_timestamp from remote subsriber_state: %w", err)
	}

	return block_timestamp, nil
}
