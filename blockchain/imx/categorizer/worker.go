// imx smartcontract cateogirzer
// for documentation see:
// https://github.com/immutable/imx-core-sdk-golang/blob/6541766b54733580889f5051653d82f077c2aa17/imx/api/docs/TransfersApi.md#ListTransfers
// https://github.com/immutable/imx-core-sdk-golang/blob/6541766b54733580889f5051653d82f077c2aa17/imx/api/docs/MintsApi.md#listmints
package categorizer

import (
	"fmt"
	"os"
	"time"

	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/app/remote/message"
	spaghetti_log "github.com/blocklords/gosds/blockchain/event"
	"github.com/blocklords/gosds/categorizer"
	"github.com/blocklords/gosds/categorizer/smartcontract"
)

// we fetch transfers and mints.
// each will slow down the sleep time to the IMX open client API.
const IMX_REQUEST_TYPE_AMOUNT = 2

// Run the goroutine for each Imx smartcontract.
func (manager *Manager) categorize(sm *smartcontract.Smartcontract) {
	url := "spaghetti_" + manager.network.Id
	sock := remote.InprocRequestSocket(url)

	// if there are some logs, we should broadcast them to the SDS Categorizer
	pusher, err := categorizer.NewCategorizerPusher()
	if err != nil {
		panic(err)
	}
	defer pusher.Close()

	addresses := []string{sm.Address}

	for {
		new_logs, err := spaghetti_log.RemoteLogFilter(sock, sm.CategorizedBlockTimestamp, addresses)
		if err != nil {
			fmt.Println("failed to get the remote block number for network: " + sm.NetworkId + " error: " + err.Error())
			fmt.Fprintf(os.Stderr, "Error when imx client for logs`: %v\n", err)
			fmt.Println("trying to request again in 10 seconds...")
			time.Sleep(10 * time.Second)
			continue
		}

		block_number, block_timestamp := spaghetti_log.RecentBlock(new_logs)
		sm.SetBlockParameter(block_number, block_timestamp)

		smartcontracts := []*smartcontract.Smartcontract{sm}

		push := message.Request{
			Command: "",
			Parameters: map[string]interface{}{
				"smartcontracts": smartcontracts,
				"logs":           new_logs,
			},
		}
		request_string, _ := push.ToString()

		_, err = pusher.SendMessage(request_string)
		if err != nil {
			panic(err)
		}

		time.Sleep(time.Second * 1)
	}
}
