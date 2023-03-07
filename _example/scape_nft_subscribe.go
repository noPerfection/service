package main

import (
	"fmt"

	"github.com/blocklords/sds/app/env"
	"github.com/blocklords/sds/categorizer"
	"github.com/blocklords/sds/common/topic"
	"github.com/blocklords/sds/sdk"
	"github.com/blocklords/sds/security"
)

func main() {
	security.EnableSecurity()
	env.LoadAnyEnv()

	// ScapeNFT topic filter
	filter := topic.TopicFilter{
		Organizations:  []string{"seascape"},
		Smartcontracts: []string{"ScapeNFT"},
		Methods:        []string{"transfer"},
	}

	subscriber, _ := sdk.NewSubscriber(filter)

	// Started Subscriber will start the fetch the data
	// the data is avaiable at subscrber.BroadcastChan channel
	subscriber.Start()

	for {
		response := <-subscriber.BroadcastChan

		if !response.IsOK() {
			fmt.Println("received an error %s", response.Reply().Message)
			break
		}

		parameters := response.Reply().Params
		transactions := parameters["transactions"].([]*categorizer.Transaction)

		fmt.Println("the transaction in the gosds/categorizer.Transaction struct", transactions)

		for _, tx := range transactions {
			nft_id := tx.Args["_nftId"]
			from := tx.Args["_from"]
			to := tx.Args["_to"]

			fmt.Println("NFT %d transferred from %s to %s", nft_id, from, to)
			fmt.Println("on a network %s at %d", tx.NetworkId, tx.BlockTimestamp)

			// Do something with the transactions
		}

	}
}
