// imx smartcontract cateogirzer
// for documentation see:
// https://github.com/immutable/imx-core-sdk-golang/blob/6541766b54733580889f5051653d82f077c2aa17/imx/api/docs/TransfersApi.md#ListTransfers
// https://github.com/immutable/imx-core-sdk-golang/blob/6541766b54733580889f5051653d82f077c2aa17/imx/api/docs/MintsApi.md#listmints
package categorizer

import (
	"sync"
	"time"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"
	spaghetti_log "github.com/blocklords/sds/blockchain/event"
	blockchain_process "github.com/blocklords/sds/blockchain/inproc"
	"github.com/blocklords/sds/categorizer/event"
	"github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
)

// Run the goroutine for each Imx smartcontract.
func (manager *Manager) categorize(sm *smartcontract.Smartcontract) {
	var mu sync.Mutex
	url := blockchain_process.BlockchainManagerUrl(manager.network.Id)
	sock, err := remote.InprocRequestSocket(url, manager.logger, manager.app_config)
	if err != nil {
		manager.logger.Fatal("remote.InprocRequest", "url", url, "error", err)
	}

	addresses := []string{sm.SmartcontractKey.Address}

	for {
		raw_logs, _, err := spaghetti_log.RemoteLogFilter(sock, blockchain.Number(sm.BlockHeader.Timestamp.Value())+1, addresses)
		if err != nil {
			manager.logger.Warn("imx remote filter (sleep and try again)", "sleep", 10*time.Second, "error", err)
			time.Sleep(10 * time.Second)
			continue
		}
		if len(raw_logs) == 0 {
			time.Sleep(1 * time.Second)
			continue
		}

		recent_block := spaghetti_log.RecentBlock(raw_logs)
		sm.SetBlockHeader(recent_block)

		new_logs := make([]*event.Log, len(raw_logs))
		for i, raw_log := range raw_logs {
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
			log.AddMetadata(raw_log).AddSmartcontractData(sm)

			new_logs[i] = log
		}

		smartcontracts := []*smartcontract.Smartcontract{sm}

		push := message.Request{
			Command: "",
			Parameters: map[string]interface{}{
				"smartcontracts": smartcontracts,
				"logs":           new_logs,
			},
		}
		request_string, _ := push.ToString()

		// pusher is a single for all categorizers
		// its defined in sds/blockchain package on a different goroutine
		// Without mutexes, the socket behaviour is undefined
		mu.Lock()
		_, err = manager.pusher.SendMessage(request_string)
		mu.Unlock()
		if err != nil {
			manager.logger.Fatal("push.SendMessage", "error", err)
		}

		time.Sleep(time.Second * 1)
	}
}
