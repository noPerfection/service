# SeascapeSDS Guide
> this is the golang package of the SeascapeSDS

***S**eascape **S**oftware **D**evelopment **S**ervice*
is the right toolbox to build feature rich applications on a blockchain.

---
Whenever you write a dapp, you also write the additional tools around the smartcontracts.

* You write an unnecessary software that frequently reads the blockchain to update your backend.
* You write an unnecessary tool that signs the transaction to change the state of smartcontract.
* You need to write calculations for metadata. Such as representing token in fiat currency, or calculating APY/APR for defi project as we faced in during mini-game development.

These tools are not exactly blockchain related. Most of the smartcontract developers doesn't required to write them. Its the burden of the backend developers.

You would be amazed how many backend developers fail during the development of these basic tools. Surprisingly it requires a good knowledge of the blockchain's API and internal work. Yet, the learning curve is quite long and painful.

Knowing these facts, there are popping a lot of startups that provides these tools for a fee. How many messages I am getting every day on my professional email, or personal email from outsourcing companies that tries to get overpriced money for such tools.

#### Let me give you more examples!


What if your application is cross-chain, let's say your NFT or Token is cross-chain. 

Or you want to utilize additional features in your smartcontracts, maybe oracles or schedulers. In that case each of them has their own cryptocurrency. You have to manage multi-currency for your single dapp.

You still wonder, why there is no big "play2earn" games and dapps?

> It comes from the expertise of the game developers working in the crypto space since 2018.

---

# Enter SeascapeSDS
Consider SeascapeSDS as a collection of microservices. You deploy the smartcontract with it, and all the tools necessary to build your dapps are magically appear to you. Each tool is microservice.

Since SeascapeSDS is in the microservice architecture, if you don't have the feature that you want, then you can create it on your own or ask the community to build it for you through bounties and share it with all other developers as we do it with you.

For big innovations, working as a single team, trying to earn money on your cryptocurrency is one of the major drawbacks that pushes the crypto space from innovation.

Right, let the cryptocurrency of each project "go to the moon" because of its popularity and its users, not because of the underlying technology.


# Example
Let's assume that the smartcontract developer deployed the smartcontract on a blockchain. He did it using SDS CLI. Now our smartcontract is registered on SeascapeSDS.

For example let's work with ScapeNFT. Its registered on the SeascapeSDS as:


```javascript

organization: "seascape"
project: "core"
network_ids: ["1", "56", "1284"]
group: "nft"
name: "ScapeNFT"
```

ScapeNFTs created by "seascape" organization. Its part of its core project. ScapeNFT belongs to the "nft" smartcontract groups.

Finally its deployed on three blockchains: `Ethereum`, `BNB Chain`, and `Moonriver`.


## Example 1: Track the ScapeNFT transfers

Create an empty project with go programming language:

```sh
?> mkdir scape_nft_example
?> go init mod
?> go get github.com/blocklords/gosds
```

With the gosds package installed, let's create the `.env` file with the authentication parameters.

> Installation process of gosds and its setup requirements will be added later.

Here is the example of tracking transactions:

```
package main

import (
	"github.com/blocklords/gosds/categorizer"
	"github.com/blocklords/gosds/env"
	"github.com/blocklords/gosds/message"
	"github.com/blocklords/gosds/sdk"
	"github.com/blocklords/gosds/security"
	"github.com/blocklords/gosds/topic"
)

func main() {
	security.EnableSecurity()
	env.LoadAnyEnv()

	// ScapeNFT topic filter
	filter := topic.TopicFilter{
            Organizations:  []string{"seascape"},
            Projects:       []string{"core"},
            Smartcontracts: []string{"ScapeNFT"},
            Methods:        []string{"transfer"},
	}

	subscriber, _ := sdk.NewSubscriber("sample", &filter, true)
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

```

That's all! No need to know what is the smartcontract address, to keep the ABI interface (If you know what are these terms mean).

SeascapeSDS will care about the network issues, about smartcontract ABI and its address.

#### Now let's discuss about about the code.

Very important thing there is the topic `filter` variable.
In the topic, we listed the smartcontract name: `ScapeNFT`, but we didn't list the network ids (remember that the NFT is deployed on `Ethereum`, `BNB Chain` and `Moonriver`).

By omitting network ids, Scape NFT on any network will be received by the backend.

If you want for example to track ScapeNFTs on BNB Chain then change the topic filter to:

```go
filter := topic.TopicFilter{
    Organizations:  []string{"seascape"},
    Projects:       []string{"core"},
    Smartcontracts: []string{"ScapeNFT"},
    NetworkIds:     []string{"1"},
    Methods:        []string{"transfer"},
}
```

* If you want to track any transaction, then remove the Methods.
* If you want to track any nft in the seascape ecosystem, then 1. delete the `Smartcontracts`, `Projects`, add the `Groups: []string{"nft"}`.

Once we got the transactions, what about the parameters of the transactions? In the example above we listed three arguments as:

```go
nft_id := tx.Args["_nftId"]
from := tx.Args["_from"]
to := tx.Args["_to"]
```

The names of the arguments are identical how they are written in the source code. 

On the roadmap, we have a plan want to generate a documentation by AI. AI will parse the smartcontract interface, and will set the basic use cases with `copy-paste` code. Write, the less developer writes, the better it is.

> More examples are coming soon.

---

# SeascapeSDS Core
This go module contains the core features
and SDK along together.

This repository isn't enough to run the SeascapeSDS in your machine. 

The following set ups are necessary for running on your machine:

* [Vault](https://vaultproject.io/) for keeping credentials
* [Mysql Database](https://mysql.com/) 
* [sds-ts](https://github.com/blocklords/sds-ts/) keeps the other core services that are written in Typescript.
* .env with the SeascapeSDS Service ports, its configuration, vault access and database parameters.

## Setup
SeascapeSDS if its running for the first time, will setup the database for you.

If the .env are not set, then it will use the default values.