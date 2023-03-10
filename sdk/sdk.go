/*
The gosds/sdk package is the client package to interact with SDS.
The following commands are available in this SDK:

1. Subscribe - subscribes for events
2. Sign - sends a transaction to the blockchain
3. AddToPool - send a transaction to the pool that will be broadcasted to the blockchain bundled.
4. Read - read a smartcontract information

# Requrements

1. GATEWAY_HOST environment variable
2. GATEWAY_PORT environment variable
3. GATEWAY_BROADCAST_HOST environment variable
4. GATEWAY_BROADCAST_PORT environment variable

# Usage

----------------------------------------------------------------
example of reading smartcontract data

	   import (
		"github.com/blocklords/sds/sdk"
		"github.com/blocklords/sds/common/topic"
	   )

	   func test() {
		topic := topic.NewTopicFilterFromString("")
		reader := sdk.NewSubscriber("address", "gateway repUrl")

		// returns gosds.message.Reply
		reply := reader.Read(importAddressTopic, args)

		if !reply.IsOk() {
			panic(fmt.Errorf("failed to read smartcontract data: %w", reply.Message))
		}

		fmt.Println("The user's address is: ", reply.Parameters["result"].(string))
	   }

-------------------------------------------

example of using Subscribe

	   func(test) {
			topicFilter := topic.TopicFilter{}
			subscriber := sdk.NewSubscriber(topicFilter)

			// first it will get the snapshots
			// then it will return the data
			err := subscriber.Start()

			if err := nil {
				panic(err)
			}

			// catch channel data
	   }
*/
package sdk

import (
	"errors"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/common/topic"
	"github.com/blocklords/sds/sdk/reader"
	"github.com/blocklords/sds/sdk/subscriber"
	"github.com/blocklords/sds/sdk/writer"
)

var Version string = "Seascape GoSDS version: 0.0.8"

// Returns a new reader.Reader.
//
// The repUrl is the link to the SDS Gateway.
// The address argument is the wallet address that is allowed to read.
//
//	address is the whitelisted user's address.
func NewReader(address string) (*reader.Reader, error) {
	e, err := gateway_service()
	if err != nil {
		return nil, err
	}

	developer_service, err := developer_service()
	if err != nil {
		return nil, err
	}

	gatewaySocket, err := remote.NewTcpSocket(e, developer_service, true)
	if err != nil {
		return nil, err
	}

	return reader.NewReader(gatewaySocket, address), nil
}

func NewWriter(address string) (*writer.Writer, error) {
	e, err := gateway_service()
	if err != nil {
		return nil, err
	}

	developer_service, err := developer_service()
	if err != nil {
		return nil, err
	}

	gatewaySocket, err := remote.NewTcpSocket(e, developer_service, true)
	if err != nil {
		return nil, err
	}

	return writer.NewWriter(gatewaySocket, address), nil
}

// Returns a new subscriber
func NewSubscriber(topic_filter topic.TopicFilter) (*subscriber.Subscriber, error) {
	e, err := gateway_service()
	if err != nil {
		return nil, err
	}

	developer_service, err := developer_service()
	if err != nil {
		return nil, err
	}

	return subscriber.NewSubscriber(&topic_filter, e, developer_service)
}

// Returns the gateway environment variable
// If the broadcast argument set true, then Gateway will require the broadcast to be set as well.
func gateway_service() (*service.Service, error) {
	e, err := service.NewSecure(service.GATEWAY, service.REMOTE)
	if err != nil {
		return nil, err
	}

	return e, nil
}

func developer_service() (*service.Service, error) {
	e, err := service.NewSecure(service.DEVELOPER_GATEWAY, service.REMOTE, service.SUBSCRIBE)
	if err != nil {
		return nil, err
	}
	if len(e.SecretKey) == 0 || len(e.PublicKey) == 0 {
		return nil, errors.New("missing 'DEVELOPER_SECRET_KEY' and/or 'DEVELOPER_PUBLIC_KEY' environment variables")
	}

	return e, nil
}
