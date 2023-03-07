package main

import (
	"fmt"
	"log"

	"github.com/blocklords/sds/app/env"
	"github.com/blocklords/sds/app/remote/message"
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
	subscriber.Start()

	for {
		reply := <-subscriber.Channel

		if reply.Status == message.FAIL {
			fmt.Println("received an error %s", reply.Message)
			break
		}

		fmt.Printf("client recevied %d amount of logs from SDS\n", len(reply.Parameters.Logs))

		for _, event := range reply.Parameters.Logs {
			nft_id := event.Output["_nftId"]
			from := event.Output["_from"]
			to := event.Output["_to"]

			fmt.Println("NFT %d transferred from %s to %s", nft_id, from, to)
			fmt.Println("on a network %s at %d", event.NetworkId, event.BlockTimestamp)

			// Do something with the event logs
		}

	}
}
