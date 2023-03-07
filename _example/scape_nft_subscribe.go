package main

import (
	"fmt"
	"log"

	"github.com/blocklords/sds/app/env"
	"github.com/blocklords/sds/categorizer/event"
	"github.com/blocklords/sds/common/topic"
	"github.com/blocklords/sds/sdk"
	"github.com/blocklords/sds/security"
)

func main() {
	sec := security.New(false)
	if err := sec.StartAuthentication(); err != nil {
		log.Fatalf("security: %w", err)
	}

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
			fmt.Println("received an error %s", response.Reply.Message)
			break
		}

		parameters := response.Reply.Parameters
		logs := parameters["logs"].([]*event.Log)

		fmt.Printf("client recevied %d amount of logs from SDS\n", len(logs))

		for _, event := range logs {
			nft_id := event.Output["_nftId"]
			from := event.Output["_from"]
			to := event.Output["_to"]

			fmt.Println("NFT %d transferred from %s to %s", nft_id, from, to)
			fmt.Println("on a network %s at %d", event.NetworkId, event.BlockTimestamp)

			// Do something with the logs
		}

	}
}
