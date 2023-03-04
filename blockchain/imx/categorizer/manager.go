package categorizer

import (
	"log"

	"github.com/blocklords/gosds/app/configuration"
	"github.com/blocklords/gosds/app/remote/message"
	blockchai_process "github.com/blocklords/gosds/blockchain/inproc"
	"github.com/blocklords/gosds/blockchain/network"
	"github.com/blocklords/gosds/categorizer/smartcontract"

	zmq "github.com/pebbe/zmq4"
)

type Manager struct {
	network *network.Network

	smartcontracts []*smartcontract.Smartcontract
	pusher         *zmq.Socket
}

func NewManager(app_config *configuration.Config, network *network.Network, pusher *zmq.Socket) *Manager {
	manager := &Manager{
		network:        network,
		smartcontracts: make([]*smartcontract.Smartcontract, 0),
		pusher:         pusher,
	}

	return manager
}

// Run as goroutine
func (manager *Manager) Start() {
	sock, err := zmq.NewSocket(zmq.PULL)
	if err != nil {
		panic(err)
	}

	url := blockchai_process.CategorizerManagerUrl(manager.network.Id)
	if err := sock.Connect(url); err != nil {
		log.Fatalf("trying to create categorizer for network id %s: %v", manager.network.Id, err)
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
