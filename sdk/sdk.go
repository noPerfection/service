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
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/common/topic"
	"github.com/blocklords/sds/sdk/reader"
	"github.com/blocklords/sds/sdk/subscriber"
	"github.com/blocklords/sds/sdk/writer"
	"github.com/blocklords/sds/security/credentials"
)

var Version string = "Seascape GoSDS version: 0.0.8"

// Returns a new reader.Reader.
//
// The repUrl is the link to the SDS Gateway.
// The address argument is the wallet address that is allowed to read.
//
//	address is the whitelisted user's address.
func NewReader(address string, plain bool) (*reader.Reader, error) {
	e, err := gateway_service(plain)
	if err != nil {
		return nil, err
	}

	creds, err := developer_credentials()
	if err != nil {
		return nil, err
	}

	gatewaySocket, err := remote.NewTcpSocket(e, creds)
	if err != nil {
		return nil, err
	}

	return reader.NewReader(gatewaySocket, address), nil
}

func NewWriter(address string, plain bool) (*writer.Writer, error) {
	e, err := gateway_service(plain)
	if err != nil {
		return nil, err
	}

	creds, err := developer_credentials()
	if err != nil {
		return nil, err
	}

	gatewaySocket, err := remote.NewTcpSocket(e, creds)
	if err != nil {
		return nil, err
	}

	return writer.NewWriter(gatewaySocket, address), nil
}

// Returns a new subscriber
func NewSubscriber(topic_filter topic.TopicFilter, plain bool) (*subscriber.Subscriber, error) {
	e, err := gateway_service(plain)
	if err != nil {
		return nil, err
	}

	var creds *credentials.Credentials
	if !plain {
		creds, err = developer_credentials()
		if err != nil {
			return nil, fmt.Errorf("developer_credentials: %w", err)
		}
	} else {
		err = env.LoadAnyEnv()
		if err != nil {
			return nil, fmt.Errorf("env.LoadAnyEnv: %w", err)
		}

		if !env.Exists("SDS_PUBLIC_KEY") {
			return nil, fmt.Errorf("environment varialbe SDS_PUBLIC_KEY not set")
		}

		public_key := env.GetString("SDS_PUBLIC_KEY")
		creds = credentials.New(public_key)
	}

	return subscriber.NewSubscriber(&topic_filter, creds, e)
}

// Returns the gateway environment variable
// If the broadcast argument set true, then Gateway will require the broadcast to be set as well.
func gateway_service(plain bool) (*service.Service, error) {
	var serv *service.Service
	var err error
	if !plain {
		serv, err = service.NewSecure(service.GATEWAY, service.REMOTE)
		if err != nil {
			return nil, fmt.Errorf("service.NewSecure: %w", err)
		}
	} else {
		serv, err = service.NewExternal(service.GATEWAY, service.REMOTE)
		if err != nil {
			return nil, fmt.Errorf("service.NewExternal: %w", err)
		}
	}

	return serv, nil
}

func developer_credentials() (*credentials.Credentials, error) {
	creds, err := credentials.NewFromVault("curves", "DEVELOPER_SECRET_KEY")
	if err != nil {
		return nil, err
	}

	return creds, nil
}
