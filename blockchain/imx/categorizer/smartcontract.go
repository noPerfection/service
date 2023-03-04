// imx smartcontract cateogirzer
// for documentation see:
// https://github.com/immutable/imx-core-sdk-golang/blob/6541766b54733580889f5051653d82f077c2aa17/imx/api/docs/TransfersApi.md#ListTransfers
// https://github.com/immutable/imx-core-sdk-golang/blob/6541766b54733580889f5051653d82f077c2aa17/imx/api/docs/MintsApi.md#listmints
package categorizer

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/app/remote/message"
	spaghetti_log "github.com/blocklords/gosds/blockchain/event"
	blockchain_process "github.com/blocklords/gosds/blockchain/inproc"
	"github.com/blocklords/gosds/categorizer/event"
	"github.com/blocklords/gosds/categorizer/smartcontract"
	"github.com/blocklords/gosds/common/data_type/key_value"
)

// Run the goroutine for each Imx smartcontract.
func (manager *Manager) categorize(sm *smartcontract.Smartcontract) {
	var mu sync.Mutex
	url := blockchain_process.BlockchainManagerUrl(manager.network.Id)
	sock := remote.InprocRequestSocket(url)

	addresses := []string{sm.Address}

	for {
		raw_logs, _, err := spaghetti_log.RemoteLogFilter(sock, sm.CategorizedBlockTimestamp+1, addresses)
		if err != nil {
			fmt.Println("failed to get the remote block number for network: " + sm.NetworkId + " error: " + err.Error())
			fmt.Fprintf(os.Stderr, "Error when imx client for logs`: %v\n", err)
			fmt.Println("trying to request again in 10 seconds...")
			time.Sleep(10 * time.Second)
			continue
		}
		if len(raw_logs) == 0 {
			fmt.Println("request result is empty for", sm.CategorizedBlockNumber, "wait for 10 seconds and try again")
			time.Sleep(10 * time.Second)
			continue
		}

		block_number, block_timestamp := spaghetti_log.RecentBlock(raw_logs)
		sm.SetBlockParameter(block_number, block_timestamp)

		new_logs := make([]*event.Log, len(raw_logs))
		for i, raw_log := range raw_logs {
			data_string := raw_log.Data
			log_kv, err := key_value.NewFromString(data_string)
			if err != nil {
				panic(err)
			}
			log_name, err := log_kv.GetString("log")
			if err != nil {
				panic(err)
			}
			log_outputs, err := log_kv.GetKeyValue("outputs")
			if err != nil {
				panic(err)
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
			panic(err)
		}

		time.Sleep(time.Second * 1)
	}
}
