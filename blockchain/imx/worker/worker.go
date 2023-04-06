package worker

import (
	"time"

	"github.com/blocklords/sds/blockchain/handler"
	"github.com/blocklords/sds/blockchain/imx"
	blockchain_proc "github.com/blocklords/sds/blockchain/inproc"

	"github.com/blocklords/sds/app/command"
	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/remote/message"
	"github.com/blocklords/sds/app/service"
	spaghetti_log "github.com/blocklords/sds/blockchain/event"
	"github.com/blocklords/sds/blockchain/imx/client"
	"github.com/blocklords/sds/common/data_type/key_value"
)

// the global variables that we pass between functions in this worker.
// the functions are recursive.
type Manager struct {
	client             *client.Client
	logger             log.Logger
	request_per_second uint64
	request_amount     uint64 // concurrent running requests
}

// A new Manager
func New(app_config *configuration.Config, client *client.Client, logger log.Logger) *Manager {
	return &Manager{
		client:             client,
		logger:             logger,
		request_per_second: app_config.GetUint64(imx.REQUEST_PER_SECOND),
		request_amount:     0,
	}
}

// Sets up the socket to interact with the clients
func (worker *Manager) SetupSocket() {
	url := blockchain_proc.ClientEndpoint(worker.client.Network.Id)
	service, err := service.InprocessFromUrl(url)
	if err != nil {
		worker.logger.Fatal("service.InprocessFromUrl", "error", err)
	}
	reply, err := controller.NewReply(service, worker.logger)
	if err != nil {
		worker.logger.Fatal("controller.NewReply", "error", err)
	}

	handlers := command.EmptyHandlers().
		Add(handler.FILTER_LOG_COMMAND, on_filter_log)

	err = reply.Run(handlers, worker)
	if err != nil {
		worker.logger.Fatal("controller.Run", "error", err)
	}
}

func on_filter_log(request message.Request, _ log.Logger, parameters ...interface{}) message.Reply {
	if parameters == nil || len(parameters) < 1 {
		return message.Fail("invalid parameters were given atleast manager should be passed")
	}

	worker, ok := parameters[0].(*Manager)
	if !ok {
		return message.Fail("missing Manager in the parameters")
	}

	var request_parameters handler.FilterLog
	err := request.Parameters.ToInterface(&request_parameters)
	if err != nil {
		return message.Fail("failed to parse request parameters: %w" + err.Error())
	}

	address := request_parameters.Addresses[0]
	block_timestamp := request_parameters.BlockFrom
	timestamp := time.Unix(int64(block_timestamp), 0).UTC().Format(time.RFC3339)

	block_timestamp_to := uint64(block_timestamp)
	timestamp_to := time.Unix(int64(block_timestamp_to), 0).UTC().Format(time.RFC3339)

	worker.request_amount++
	delay_duration := worker.delay_duration()
	transfers, err := worker.client.GetSmartcontractTransferLogs(address, delay_duration, timestamp, timestamp_to)
	if err != nil {
		worker.request_amount--
		return message.Fail("client.GetSmartcontractTransferLogs: " + err.Error())
	}
	if len(transfers) > 0 {
		recent_block := spaghetti_log.RecentBlock(transfers)
		block_timestamp_to = recent_block.Timestamp.Value()

		timestamp_to = time.Unix(int64(block_timestamp_to), 0).UTC().Format(time.RFC3339)
	}

	worker.request_amount++
	delay_duration = worker.delay_duration()
	mints, err := worker.client.GetSmartcontractMintLogs(address, delay_duration, timestamp, timestamp_to)
	if err != nil {
		worker.request_amount--
		return message.Fail("client.GetSmartcontractMingLogs: " + err.Error())
	}

	transfers = append(transfers, mints...)

	if len(transfers) > 0 {
		recent_block := spaghetti_log.RecentBlock(transfers)
		block_timestamp_to = recent_block.Timestamp.Value()
	}

	reply := message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"logs":     transfers,
			"block_to": block_timestamp_to,
		}),
	}

	return reply
}

// Based on total amount of smartcontracts, how long we delay to request to ImmutableX nodes
func (manager *Manager) delay_duration() time.Duration {
	per_second := float64(manager.request_per_second)
	amount := float64(manager.request_amount)

	return time.Duration(float64(time.Millisecond) * amount * 1000 / per_second)
}
