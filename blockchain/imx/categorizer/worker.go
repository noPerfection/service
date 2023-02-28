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
	"github.com/blocklords/gosds/categorizer/smartcontract"
	spaghetti_log "github.com/blocklords/gosds/blockchain/log"
)

// we fetch transfers and mints.
// each will slow down the sleep time to the IMX open client API.
const IMX_REQUEST_TYPE_AMOUNT = 2

// Run the goroutine for each Imx smartcontract.
func (manager *Manager) categorize(sm *smartcontract.Smartcontract) {
	url := "spaghetti_" + manager.network.Id
	sock := remote.InprocRequestSocket(url)

	for {
		addresses := []string{sm.Address}
		_, err := spaghetti_log.RemoteLogFilter(sock, sm.CategorizedBlockTimestamp, addresses)
		if err != nil {
			fmt.Println("failed to get the remote block number for network: " + sm.NetworkId + " error: " + err.Error())
			fmt.Fprintf(os.Stderr, "Error when imx client for logs`: %v\n", err)
			fmt.Println("trying to request again in 10 seconds...")
			time.Sleep(10 * time.Second)
			continue
		}

		// if there are some logs, we should broadcast them to the SDS Categorizer

		time.Sleep(time.Second * 1)
	}
}
