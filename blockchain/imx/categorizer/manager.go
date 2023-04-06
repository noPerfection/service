package categorizer

import (
	"fmt"

	"github.com/blocklords/sds/app/command"
	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"
	"github.com/blocklords/sds/blockchain/handler"
	blockchai_process "github.com/blocklords/sds/blockchain/inproc"
	"github.com/blocklords/sds/blockchain/network"
	"github.com/blocklords/sds/categorizer"
	"github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/common/data_type/key_value"

	zmq "github.com/pebbe/zmq4"
)

type Manager struct {
	network *network.Network

	smartcontracts []*smartcontract.Smartcontract
	pusher         *zmq.Socket
	logger         log.Logger
	app_config     *configuration.Config
}

func NewManager(parent log.Logger, app_config *configuration.Config, network *network.Network) (*Manager, error) {
	logger, err := parent.ChildWithTimestamp("categorizer")
	if err != nil {
		return nil, fmt.Errorf("child logger: %w", err)
	}

	manager := &Manager{
		network:        network,
		smartcontracts: make([]*smartcontract.Smartcontract, 0),
		logger:         logger,
		app_config:     app_config,
	}

	return manager, nil
}

// Run as goroutine
func (manager *Manager) Start() {
	// if there are some logs, we should broadcast them to the SDS Categorizer
	pusher, err := categorizer.NewCategorizerPusher()
	if err != nil {
		manager.logger.Fatal("create a pusher to SDS Categorizer", "message", err)
	}
	manager.pusher = pusher

	url := blockchai_process.CategorizerEndpoint(manager.network.Id)
	service, err := service.InprocessFromUrl(url)
	if err != nil {
		manager.logger.Fatal("failed to create inproc service from url", "error", err)
	}
	reply, err := controller.NewPull(service, manager.logger)
	if err != nil {
		manager.logger.Fatal("failed to create pull controller", "error", err)
	}

	handlers := command.EmptyHandlers().
		Add(handler.NEW_CATEGORIZED_SMARTCONTRACTS, on_new_smartcontracts)
	err = reply.Run(handlers, manager)
	if err != nil {
		manager.logger.Fatal("failed to run reply controller", "error", err)
	}
}

// Based on total amount of smartcontracts, how long we delay to request to ImmutableX nodes
func (manager *Manager) AddSmartcontract(sm *smartcontract.Smartcontract) {
	manager.smartcontracts = append(manager.smartcontracts, sm)
}

func on_new_smartcontracts(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	if parameters == nil || len(parameters) < 1 {
		return message.Fail("invalid parameters were given atleast manager should be passed")
	}

	manager, ok := parameters[0].(*Manager)
	if !ok {
		return message.Fail("missing Manager in the parameters")
	}

	manager.logger.Info("add new smartcontracts to the manager")

	raw_smartcontracts, _ := request.Parameters.GetKeyValueList("smartcontracts")

	for _, raw := range raw_smartcontracts {
		sm, _ := smartcontract.New(raw)

		manager.AddSmartcontract(sm)
		go manager.categorize(sm)
	}

	return message.Reply{
		Status:     message.OK,
		Message:    "",
		Parameters: key_value.Empty(),
	}
}
