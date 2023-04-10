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
	"fmt"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/service"
	service_credentials "github.com/blocklords/sds/app/service/auth"
	"github.com/blocklords/sds/common/topic"
	"github.com/blocklords/sds/sdk/reader"
	"github.com/blocklords/sds/sdk/subscriber"
	"github.com/blocklords/sds/sdk/writer"
	"github.com/blocklords/sds/security/auth"
	"github.com/blocklords/sds/security/vault"
)

var Version = "Seascape GoSDS version: 0.0.8"

type Sdk struct {
	logger log.Logger
	config *configuration.Config
}

// Returns a new reader.Reader.
//
// The repUrl is the link to the SDS Gateway.
// The address argument is the wallet address that is allowed to read.
//
//	address is the whitelisted user's address.
func (sdk *Sdk) NewReader(address string) (*reader.Reader, error) {
	e, err := sdk.gateway_service()
	if err != nil {
		return nil, err
	}

	creds, err := developer_credentials()
	if err != nil {
		return nil, err
	}

	gatewaySocket, err := remote.NewTcpSocket(e, sdk.logger, sdk.config)
	if err != nil {
		return nil, err
	}
	if creds != nil {
		service_creds, err := service_credentials.ServiceCredentials(service.GATEWAY, service.REMOTE, sdk.config)
		if err != nil {
			return nil, err
		}
		gatewaySocket.SetSecurity(service_creds.PublicKey, creds)
	}

	return reader.NewReader(gatewaySocket, address), nil
}

func (sdk *Sdk) NewWriter(address string) (*writer.Writer, error) {
	e, err := sdk.gateway_service()
	if err != nil {
		return nil, err
	}

	creds, err := developer_credentials()
	if err != nil {
		return nil, err
	}

	gatewaySocket, err := remote.NewTcpSocket(e, sdk.logger, sdk.config)
	if err != nil {
		return nil, err
	}
	if creds != nil {
		service_creds, err := service_credentials.ServiceCredentials(service.GATEWAY, service.REMOTE, sdk.config)
		if err != nil {
			return nil, err
		}
		gatewaySocket.SetSecurity(service_creds.PublicKey, creds)
	}

	return writer.NewWriter(gatewaySocket, address), nil
}

// Returns a new subscriber
func (sdk *Sdk) NewSubscriber(topic_filter topic.TopicFilter) (*subscriber.Subscriber, error) {
	e, err := sdk.gateway_service()
	if err != nil {
		return nil, err
	}

	var creds *auth.Credentials
	if sdk.config.Secure {
		creds, err = developer_credentials()
		if err != nil {
			return nil, fmt.Errorf("developer_credentials: %w", err)
		}
	} else {
		if !sdk.config.Exist("SDS_PUBLIC_KEY") {
			return nil, fmt.Errorf("environment varialbe SDS_PUBLIC_KEY not set")
		}

		public_key := sdk.config.GetString("SDS_PUBLIC_KEY")
		creds = auth.New(public_key)
	}

	return subscriber.NewSubscriber(&topic_filter, creds, e, sdk.logger, sdk.config)
}

// Returns the gateway environment variable
// If the broadcast argument set true, then Gateway will require the broadcast to be set as well.
func (sdk *Sdk) gateway_service() (*service.Service, error) {
	var serv *service.Service
	var err error
	if sdk.config.Secure {
		serv, err = service.NewExternal(service.GATEWAY, service.REMOTE, sdk.config)
		if err != nil {
			return nil, fmt.Errorf("service.NewSecure: %w", err)
		}
	} else {
		serv, err = service.NewExternal(service.GATEWAY, service.REMOTE, sdk.config)
		if err != nil {
			return nil, fmt.Errorf("service.NewExternal: %w", err)
		}
	}

	return serv, nil
}

func NewSdk() (*Sdk, error) {
	logger, err := log.New("seascape-sdk", log.WITH_TIMESTAMP)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize sds log engine: %w", err)
	}
	app_config, err := configuration.NewAppConfig(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize the sdk configuration engine: %w", err)
	}

	return &Sdk{
		logger: logger,
		config: app_config,
	}, nil
}

func developer_credentials() (*auth.Credentials, error) {
	creds, err := vault.GetCredentials("SDS", "DEVELOPER_SECRET_KEY")
	if err != nil {
		return nil, err
	}

	return creds, nil
}
