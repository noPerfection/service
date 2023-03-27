package categorizer

import (
	"fmt"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote/message"
	blockchai_process "github.com/blocklords/sds/blockchain/inproc"
	"github.com/blocklords/sds/blockchain/network"
	"github.com/blocklords/sds/categorizer/smartcontract"

	zmq "github.com/pebbe/zmq4"
)

type Manager struct {
	network *network.Network

	smartcontracts []*smartcontract.Smartcontract
	pusher         *zmq.Socket
	logger         log.Logger
	app_config     *configuration.Config
}

func NewManager(parent log.Logger, app_config *configuration.Config, network *network.Network, pusher *zmq.Socket) (*Manager, error) {
	logger, err := parent.ChildWithTimestamp("categorizer")
	if err != nil {
		return nil, fmt.Errorf("child logger: %w", err)
	}

	manager := &Manager{
		network:        network,
		smartcontracts: make([]*smartcontract.Smartcontract, 0),
		pusher:         pusher,
		logger:         logger,
		app_config:     app_config,
	}

	return manager, nil
}

// Run as goroutine
func (manager *Manager) Start() {
	sock, err := zmq.NewSocket(zmq.PULL)
	if err != nil {
		manager.logger.Fatal("new pull socket", "error", err)
	}

	url := blockchai_process.CategorizerManagerUrl(manager.network.Id)
	if err := sock.Connect(url); err != nil {
		manager.logger.Fatal("socket.Connect", "error", err)
	}

	for {
		// Wait for reply.
		msgs, _ := sock.RecvMessage(0)
		request, _ := message.ParseRequest(msgs)

		raw_smartcontracts, _ := request.Parameters.GetKeyValueList("smartcontracts")

		for _, raw := range raw_smartcontracts {
			sm, _ := smartcontract.New(raw)

			manager.AddSmartcontract(sm)
			go manager.categorize(sm)
		}
	}
}

// Based on total amount of smartcontracts, how long we delay to request to ImmutableX nodes
func (manager *Manager) AddSmartcontract(sm *smartcontract.Smartcontract) {
	manager.smartcontracts = append(manager.smartcontracts, sm)
}
