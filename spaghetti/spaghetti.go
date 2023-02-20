// The SDS Spaghetti module fetches the blockchain data and converts it into the internal format
// All other SDS Services are connecting to SDS Spaghetti.
//
// We have multiple workers.
// Atleast one worker for each network.
// This workers are called recent workers.
//
// The recent workers are caching the block information by env.SDS_SPAGHETTI_CACHE_DURATION seconds
//
// Categorizer checks whether the cached block returned or not.
// If its a cached block, then switches to the block_range
package spaghetti

import (
	"database/sql"
	"fmt"
	debug_log "log"
	"strings"

	"github.com/blocklords/gosds/spaghetti/block"
	"github.com/blocklords/gosds/spaghetti/log"
	"github.com/blocklords/gosds/spaghetti/network_client"
	"github.com/blocklords/gosds/spaghetti/transaction"
	"github.com/blocklords/gosds/spaghetti/worker"
	"github.com/blocklords/gosds/static/network"

	"github.com/blocklords/gosds/security/vault"

	"github.com/blocklords/gosds/app/configuration"
	"github.com/blocklords/gosds/app/service"

	"github.com/blocklords/gosds/app/account"
	"github.com/blocklords/gosds/app/argument"
	"github.com/blocklords/gosds/app/broadcast"
	"github.com/blocklords/gosds/app/controller"
	"github.com/blocklords/gosds/app/remote"
	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/common/data_type"
	"github.com/blocklords/gosds/common/data_type/key_value"
	"github.com/blocklords/gosds/db"
	"github.com/blocklords/gosds/security"
)

var static_socket *remote.Socket
var network_clients map[string]*network_client.NetworkClient
var workers []*worker.SpaghettiWorker

func run_each_evm_network_sync_worker(dbCon *db.Database, broadcast_channel chan message.Broadcast, debug bool) {
	workers = make([]*worker.SpaghettiWorker, 0)
	for _, client := range network_clients {
		recent_block_number, err := block.GetRecentBlockNumber(dbCon, client.Network.Id)
		if err != nil {
			if err == sql.ErrNoRows {
				println("detected a new network: ", client.Network.Id)
				recent_block_number, err = client.GetRecentBlockNumber()
				if err != nil {
					panic(err)
				}
				recent_block, err := client.GetBlock(recent_block_number)
				if err != nil {
					panic(err)
				}

				fmt.Printf("SDS Spaghetti starts to count the network id %s from the block number: %v\n", client.Network.Id, recent_block_number)

				err = block.SetBlock(dbCon, client.Network.Id, recent_block_number, uint(len(recent_block.Transactions)), uint(len(recent_block.Logs)), recent_block.BlockTimestamp)
				if err != nil {
					panic("SDS Spaghetti error to init a new network: " + err.Error())
				}
				transaction_err := worker.SaveTransactions(dbCon, recent_block.Transactions)
				if transaction_err != nil {
					panic(transaction_err)
				}

				log_err := worker.SaveLogs(dbCon, recent_block.Logs)
				if log_err != nil {
					panic(log_err)
				}
			} else {
				panic(err)
			}
		}

		sync_bot := worker.New(client, recent_block_number, dbCon, broadcast_channel, debug)
		go sync_bot.Sync()
		workers = append(workers, sync_bot)
	}
}

////////////////////////////////////////////////////////////////////
//
// Command handlers
//
////////////////////////////////////////////////////////////////////

// this function returns a spaghetti block, including
//
// - mined timestamp
//
// - transactions
//
// - logs
//
// additional parameter that it takes is "address"
// you can fetch transactions and logs happened with a certain smartcontract.
func block_get(db *db.Database, request message.Request) message.Reply {
	network_id, err := request.Parameters.GetString("network_id")
	if err != nil {
		return message.Fail(err.Error())
	}
	block_number, err := request.Parameters.GetUint64("block_number")
	if err != nil {
		return message.Fail(err.Error())
	}
	address, _ := request.Parameters.GetString("to")

	recent_block_number, err := block.GetRecentBlockNumber(db, network_id)
	if err != nil {
		return message.Fail(err.Error())
	}
	if block_number > recent_block_number {
		return message.Fail(fmt.Sprintf("invalid block number. block number should be less or equal to %d", recent_block_number))
	}
	earliest_block_number, err := block.GetEarliestBlockNumber(db, network_id)
	if err != nil {
		return message.Fail(err.Error())
	}

	var timestamp uint64
	var transactions []*transaction.Transaction
	var logs []*log.Log

	cached := false

	if block_number >= earliest_block_number {
		cached = true
		timestamp, err = block.GetBlockTimestamp(db, network_id, block_number)
		if err != nil {
			return message.Fail(err.Error())
		}

		if len(address) > 0 {
			transactions, err = transaction.GetForBlockAndTxTo(db, network_id, block_number, address)
			if err != nil {
				return message.Fail(err.Error())
			}
			logs, err = log.GetForBlockAndTxTo(db, network_id, block_number, block_number, address)
			if err != nil {
				return message.Fail(err.Error())
			}
		} else {
			transactions, err = transaction.GetForBlock(db, network_id, block_number)
			if err != nil {
				return message.Fail(err.Error())
			}
			logs, err = log.GetForBlock(db, network_id, block_number)
			if err != nil {
				return message.Fail(err.Error())
			}
		}
	} else {
		client, ok := network_clients[network_id]
		if !ok {
			return message.Fail("the worker for a network id is not set. possibly invalid network_id parameter")
		}
		block, err := client.GetBlock(block_number)
		if err != nil {
			return message.Fail(err.Error())
		}

		timestamp = block.BlockTimestamp

		if len(address) > 0 {
			transactions = make([]*transaction.Transaction, 0)
			logs = make([]*log.Log, 0)

			for _, transaction := range block.Transactions {
				if !strings.EqualFold(transaction.TxTo, address) {
					continue
				}
				transactions = append(transactions, transaction)

				for _, log := range block.Logs {
					if strings.EqualFold(log.Txid, transaction.Txid) {
						logs = append(logs, log)
					}
				}
			}
		} else {
			transactions = block.Transactions
			logs = block.Logs
		}
	}

	return message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"cached":       cached,
			"network_id":   network_id,
			"block_number": block_number,
			"to":           address,
			"timestamp":    timestamp,
			"transactions": data_type.ToMapList(transactions),
			"logs":         data_type.ToMapList(logs),
		}),
	}
}

// returns the earliest cached block number
func block_get_cached_number(db *db.Database, request message.Request) message.Reply {
	network_id, err := request.Parameters.GetString("network_id")
	if err != nil {
		return message.Fail(err.Error())
	}

	earliest_block_number, err := block.GetEarliestBlockNumber(db, network_id)
	if err != nil {
		return message.Fail(err.Error())
	}

	recent_block_number, err := block.GetRecentBlockNumber(db, network_id)
	if err != nil {
		return message.Fail(err.Error())
	}

	if earliest_block_number == 0 || recent_block_number == 0 {
		return message.Fail("the cached block number is 0")
	}
	if earliest_block_number == recent_block_number {
		return message.Fail(`the cached block number keeps one block which is not enough. consider increasing value of 'SDS_SPAGHETTI_CACHE_DURATION' environment variable and restart SDS Spaghetti`)
	}

	cached_block_number := earliest_block_number + (recent_block_number / earliest_block_number)

	cached_block_timestamp, err := block.GetBlockTimestamp(db, network_id, cached_block_number)
	if err != nil {
		return message.Fail(err.Error())
	}

	return message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"network_id":      network_id,
			"block_number":    cached_block_number,
			"block_timestamp": cached_block_timestamp,
		}),
	}
}

// Returns the block timestamp
func block_get_timestamp(db *db.Database, request message.Request) message.Reply {
	network_id, err := request.Parameters.GetString("network_id")
	if err != nil {
		return message.Fail(err.Error())
	}
	block_number, err := request.Parameters.GetUint64("block_number")
	if err != nil {
		return message.Fail(err.Error())
	}

	block_timestamp, err := block.GetBlockTimestamp(db, network_id, block_number)
	if err != nil {
		if err == sql.ErrNoRows {
			if network_clients[network_id] == nil {
				return message.Fail("unsupported network_id " + network_id)
			}

			println("the block timestamp for", block_number, "in network id "+network_id+" not found")
			println("SDS Spaghetti didn't cache it yet. Meanwhile getting the block timestamp from the blockchain")

			timestamp, err := network_clients[network_id].GetBlockTimestamp(block_number)
			if err != nil {
				return message.Fail(err.Error())
			}

			block_timestamp = timestamp
		} else {
			return message.Fail(err.Error())
		}
	}

	return message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"network_id":      network_id,
			"block_number":    block_number,
			"block_timestamp": block_timestamp,
		}),
	}
}

// Returns the transactions and logs in a range of the block.
// Optionally it accepts to parameter that filters the transactions and logs
// for the smartcontract.
func block_get_range(db *db.Database, request message.Request) message.Reply {
	network_id, err := request.Parameters.GetString("network_id")
	if err != nil {
		return message.Fail(err.Error())
	}
	block_numbers, err := request.Parameters.GetUint64s("block_number_from", "block_number_to")
	if err != nil {
		return message.Fail(err.Error())
	}

	to, _ := request.Parameters.GetString("to")

	earliest_block_number, err := block.GetEarliestBlockNumber(db, network_id)
	if err != nil {
		return message.Fail(err.Error())
	}
	if block_numbers[0] < earliest_block_number || block_numbers[1] < earliest_block_number {
		return message.Fail(fmt.Sprintf("please run a worker, the database keeps the blockchain data up until %d", earliest_block_number))
	}

	recent_block_number, err := block.GetRecentBlockNumber(db, network_id)
	if err != nil {
		return message.Fail(err.Error())
	}
	if block_numbers[0] > recent_block_number || block_numbers[1] > recent_block_number {
		return message.Fail(fmt.Sprintf("please run a worker, the database keeps the blockchain data up until %d", recent_block_number))
	}

	timestamp, err := block.GetBlockTimestamp(db, network_id, block_numbers[1])

	if err != nil {
		return message.Fail(err.Error())
	}

	var transactions []*transaction.Transaction
	var logs []*log.Log

	if to != "" {
		transactions, err = transaction.GetForBlockRangeAndTxTo(db, network_id, block_numbers[0], block_numbers[1], to)
		if err != nil {
			return message.Fail(err.Error())
		}

		logs, err = log.GetForBlockAndTxTo(db, network_id, block_numbers[0], block_numbers[1], to)
		if err != nil {
			return message.Fail(err.Error())
		}
	} else {
		transactions, err = transaction.GetForBlockRangeAndTx(db, network_id, block_numbers[0], block_numbers[1])
		if err != nil {
			return message.Fail(err.Error())
		}

		logs, err = log.GetForBlockAndTx(db, network_id, block_numbers[0], block_numbers[1])
		if err != nil {
			return message.Fail(err.Error())
		}
	}

	return message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"network_id":   network_id,
			"to":           to,
			"timestamp":    timestamp,
			"transactions": data_type.ToMapList(transactions),
			"logs":         data_type.ToMapList(logs),
		}),
	}
}

// this function returns the smartcontract deployer, deployed block number
// and block timestamp by a transaction hash of the smartcontract deployment.
func transaction_deployed_get(_ *db.Database, request message.Request) message.Reply {
	network_id, err := request.Parameters.GetString("network_id")
	if err != nil {
		return message.Fail(err.Error())
	}
	txid, err := request.Parameters.GetString("txid")
	if err != nil {
		return message.Fail(err.Error())
	}

	if network_clients[network_id] == nil {
		return message.Fail("unsupported network_id " + network_id)
	}

	tx, err := network_clients[network_id].GetTransaction(txid)
	if err != nil {
		return message.Fail(err.Error())
	}

	reply := message.Reply{
		Status:  "OK",
		Message: "",
		Parameters: key_value.New(map[string]interface{}{
			"network_id":      network_id,
			"block_number":    tx.BlockNumber,
			"block_timestamp": tx.BlockTimestamp,
			"address":         tx.TxTo,
			"deployer":        tx.TxFrom,
			"txid":            txid,
		}),
	}

	return reply
}

func Run(app_config *configuration.Config, db_con *db.Database, v *vault.Vault) {
	if err := security.EnableSecurity(); err != nil {
		panic(err)
	}

	arguments, err := argument.GetArguments()
	if err != nil {
		panic(err)
	}

	app_config.SetDefault("SDS_SPAGHETTI_CACHE_DURATION", 86400)
	cache_duration := app_config.GetUint64("SDS_SPAGHETTI_CACHE_DURATION")
	if cache_duration == 0 || cache_duration > 86400 {
		debug_log.Fatalf("environment variable 'SDS_SPAGHETTI_CACHE_DURATION' is invalid. should be number less than 86400 but its %d", cache_duration)
	}

	greeting := `SDS Spaghetti preparation...
It supports the following arguments:
    --broadcast         enable SDS SDS broadcaster
    --reply             runs SDS Spaghetti request-reply server
    --broadcast-debug   set it to print the spaghetti worker log
    --security-debug    set it to print the security log
    --plain             to disable the security layer`
	println(greeting)

	spaghetti_env, err := service.New(service.SPAGHETTI, service.BROADCAST, service.THIS)
	if err != nil {
		panic(err)
	}

	categorizer_env, err := service.New(service.CATEGORIZER, service.SUBSCRIBE, service.REMOTE)
	if err != nil {
		panic(err)
	}

	gateway_env, err := service.New(service.GATEWAY, service.REMOTE)
	if err != nil {
		panic(err)
	}

	static_env, err := service.New(service.STATIC, service.REMOTE)
	if err != nil {
		panic(err)
	}

	accounts := account.NewAccounts(account.NewService(categorizer_env), account.NewService(gateway_env))

	// error since no reply or broadcast were given
	if !app_config.Broadcast && !app_config.Reply {
		debug_log.Fatalf("'%s' missing --reply and/or --broadcast. Please pass it as an argument", spaghetti_env.ServiceName())
	}

	static_socket = remote.TcpRequestSocketOrPanic(static_env, spaghetti_env)
	networks, err := network.GetRemoteNetworks(static_socket, network.WITH_VM)
	if err != nil {
		panic(err)
	}

	network_clients, err = network_client.SetupClients(networks)
	if err != nil {
		panic(err)
	}

	if app_config.Broadcast {
		broadcast_debug := argument.Has(arguments, argument.BROADCAST_DEBUG)
		broadcast_channel := make(chan message.Broadcast)
		run_each_evm_network_sync_worker(db_con, broadcast_channel, broadcast_debug)

		if app_config.Reply {
			go broadcast.Run(broadcast_channel, spaghetti_env, []*service.Service{categorizer_env})
		} else {
			fmt.Println("Running SDS Spaghetti broadcaster only")
			broadcast.Run(broadcast_channel, spaghetti_env, []*service.Service{categorizer_env})
		}
	}

	if app_config.Reply {
		var commands = controller.CommandHandlers{
			"block_get_cached_number":  block_get_cached_number,
			"block_get":                block_get,
			"block_get_timestamp":      block_get_timestamp,
			"block_get_range":          block_get_range,
			"transaction_deployed_get": transaction_deployed_get,
		}
		err := controller.ReplyController(db_con, commands, spaghetti_env, accounts)
		if err != nil {
			panic(err)
		}
	}
}
