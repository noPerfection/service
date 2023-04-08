// Package categorizer is the sub serice for ImmutableX (in short 'imx') blockchain
// that decodes the smartcontracts on the immutable x.
//
// The 'imx' categorizer supports NFT and ERC20 tokens only.
//
// The imx categorizer categorizes the following types:
//   - transfer
//   - mint
//
// For reference documentation see:
// https://github.com/immutable/imx-core-sdk-golang/blob/6541766b54733580889f5051653d82f077c2aa17/imx/api/docs/TransfersApi.md#ListTransfers
// https://github.com/immutable/imx-core-sdk-golang/blob/6541766b54733580889f5051653d82f077c2aa17/imx/api/docs/MintsApi.md#listmints
package categorizer

import (
	"time"

	"github.com/blocklords/sds/app/remote"
	spaghetti_log "github.com/blocklords/sds/blockchain/event"
	"github.com/blocklords/sds/blockchain/handler"
	blockchain_process "github.com/blocklords/sds/blockchain/inproc"
	"github.com/blocklords/sds/categorizer/event"
	"github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"

	categorizer_command "github.com/blocklords/sds/categorizer/handler"
)

// Run the goroutine for each Imx smartcontract.
func (manager *Manager) categorize(sm *smartcontract.Smartcontract) {
	url := blockchain_process.ClientEndpoint(manager.network.Id)
	sock, err := remote.InprocRequestSocket(url, manager.logger, manager.app_config)
	if err != nil {
		manager.logger.Fatal("remote.InprocRequest", "url", url, "error", err)
	}

	addresses := []string{sm.SmartcontractKey.Address}

	for {

		block_number_from := blockchain.Number(sm.BlockHeader.Timestamp.Value()) + 1

		req_parameters := handler.FilterLog{
			BlockFrom: block_number_from,
			Addresses: addresses,
		}

		var parameters handler.LogFilterReply
		err = handler.FILTER_LOG_COMMAND.Request(sock, req_parameters, &parameters)
		if err != nil {
			manager.logger.Warn("imx remote filter (sleep and try again)", "sleep", 10*time.Second, "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		if len(parameters.RawLogs) == 0 {
			time.Sleep(1 * time.Second)
			continue
		}

		recent_block := spaghetti_log.RecentBlock(parameters.RawLogs)
		sm.SetBlockHeader(recent_block)

		new_logs := make([]event.Log, len(parameters.RawLogs))
		for i, raw_log := range parameters.RawLogs {
			data_string := raw_log.Data
			log_kv, err := key_value.NewFromString(data_string)
			if err != nil {
				manager.logger.Fatal("raw_log.Data() -> key_value", "error", err)
			}
			log_name, err := log_kv.GetString("log")
			if err != nil {
				manager.logger.Fatal("log.GetString(name)", "error", err)
			}
			log_outputs, err := log_kv.GetKeyValue("outputs")
			if err != nil {
				manager.logger.Fatal("log.GetKeyValue outputs", "error", err)
			}
			log := event.New(log_name, log_outputs)
			log.AddMetadata(&raw_log).AddSmartcontractData(sm)

			new_logs[i] = *log
		}

		// pusher is a single for all categorizers
		// its defined in sds/blockchain package on a different goroutine
		// Without mutexes, the socket behaviour is undefined

		request := categorizer_command.PushCategorization{
			Smartcontracts: []smartcontract.Smartcontract{*sm},
			Logs:           new_logs,
		}
		err = categorizer_command.CATEGORIZATION.Push(manager.pusher, request)

		if err != nil {
			manager.logger.Fatal("push.SendMessage", "error", err)
		}

		time.Sleep(time.Second * 1)
	}
}
