/*
The gosds/sdk package is the client package to interact with SDS.
The following commands are available in this SDK:

1. Subscribe - subscribe for events
2. Sign - send a transaction to the blockchain
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
		"github.com/blocklords/gosds/sdk"
		"github.com/blocklords/gosds/common/topic"
	   )

	   func test() {
		// returns sdk.Reader
		reader := sdk.NewReader("address", "gateway repUrl")
		// gosds.topic.Topic
		importAddressTopic := topic.ParseString("metaking.blocklords.11155111.transfer.ImportExportManager.accountHodlerOf")
		args := ["user address"]

		// returns gosds.message.Reply
		reply := reader.Read(importAddressTopic, args)

		if !reply.IsOk() {
			panic(fmt.Errorf("failed to read smartcontract data: %w", reply.Message))
		}

		fmt.Println("The user's address is: ", reply.Params["result"].(string))
	   }

-------------------------------------------

example of using Subscribe

	   func(test) {
			topicFilter := topic.TopicFilter{}
			subscriber := sdk.NewSubscriber("address", topicFilter)

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

	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/app/service"
	"github.com/blocklords/gosds/common/topic"
	"github.com/blocklords/gosds/sdk/db"
	"github.com/blocklords/gosds/sdk/reader"
	"github.com/blocklords/gosds/sdk/subscriber"
	"github.com/blocklords/gosds/sdk/writer"
)

var Version string = "Seascape GoSDS version: 0.0.8"

// Returns a new reader.Reader.
//
// The repUrl is the link to the SDS Gateway.
// The address argument is the wallet address that is allowed to read.
//
//	address is the whitelisted user's address.
func NewReader(address string) (*reader.Reader, error) {
	e, err := gatewayEnv(false)
	if err != nil {
		return nil, err
	}

	developer_env, err := developer_env()
	if err != nil {
		return nil, err
	}

	gatewaySocket := remote.TcpRequestSocketOrPanic(e, developer_env)

	return reader.NewReader(gatewaySocket, address), nil
}

func NewWriter(address string) (*writer.Writer, error) {
	e, err := gatewayEnv(false)
	if err != nil {
		return nil, err
	}

	developer_env, err := developer_env()
	if err != nil {
		return nil, err
	}

	gatewaySocket := remote.TcpRequestSocketOrPanic(e, developer_env)

	return writer.NewWriter(gatewaySocket, address), nil
}

// Returns a new subscriber
func NewSubscriber(address string, topicFilter *topic.TopicFilter, clear_cache bool) (*subscriber.Subscriber, error) {
	e, err := gatewayEnv(true)
	if err != nil {
		return nil, err
	}

	developer_env, err := developer_env()
	if err != nil {
		return nil, err
	}

	gatewaySocket := remote.TcpRequestSocketOrPanic(e, developer_env)

	db, err := db.OpenKVM(topicFilter)
	if err != nil {
		return nil, err
	}

	return subscriber.NewSubscriber(gatewaySocket, db, address, clear_cache)
}

// Returns the gateway environment variable
// If the broadcast argument set true, then Gateway will require the broadcast to be set as well.
func gatewayEnv(broadcast bool) (*service.Service, error) {
	e, err := service.New(service.GATEWAY, service.REMOTE, service.SUBSCRIBE)
	if err != nil {
		return nil, err
	}

	return e, nil
}

func developer_env() (*service.Service, error) {
	e, err := service.New(service.DEVELOPER_GATEWAY, service.REMOTE, service.SUBSCRIBE)
	if err != nil {
		return nil, err
	}
	if len(e.SecretKey) == 0 || len(e.PublicKey) == 0 {
		return nil, errors.New("missing 'DEVELOPER_SECRET_KEY' and/or 'DEVELOPER_PUBLIC_KEY' environment variables")
	}

	return e, nil
}
