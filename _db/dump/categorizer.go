package main

import (
	"strconv"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/configuration/argument"
	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/blockchain/network"
	"github.com/blocklords/sds/categorizer/event"
	"github.com/blocklords/sds/categorizer/smartcontract"
	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/data_type/key_value"
	"github.com/blocklords/sds/db"

	faker "github.com/brianvoe/gofakeit/v6"
)

func main() {
	logger, _ := log.New("dump", log.WITHOUT_TIMESTAMP)

	// connect database
	app_config, _ := configuration.NewAppConfig(logger)
	app_config.SetDefaults(db.DatabaseConfigurations)
	db_parameters, _ := db.GetParameters(app_config)
	db_credentials := db.GetDefaultCredentials(app_config)
	db_con, _ := db.Open(logger, db_parameters, db_credentials)

	// load smartcontract
	// we test with EVM only
	networks, _ := network.GetNetworks(network.EVM)
	if len(networks) == 0 {
		logger.Fatal("no networks", "network type", network.EVM)
	}
	smartcontracts, _ := smartcontract.GetAllByNetworkId(db_con, networks[0].Id)
	if len(smartcontracts) == 0 {
		logger.Fatal("no smartcontracts", "network_id", networks[0].Id)
	}

	// now we set the amount of dump data
	if !argument.Exist("amount") {
		logger.Fatal("no --amount argument")
	}
	args := argument.GetArguments(&logger)
	amount_string, _ := argument.ExtractValue(args, "amount")
	amount, err := strconv.ParseUint(amount_string, 10, 16)
	if err != nil {
		logger.Fatal("parse --amount argument", "argument", amount_string, "error", err)
	}
	if amount == 0 {
		logger.Fatal("no --amount argument is 0")
	}

	for _, sm := range smartcontracts {
		for i := uint64(0); i < amount; i++ {
			fake_parameters := key_value.Empty().
				Set("from", faker.Username()).
				Set("to", faker.Username()).
				Set("value", faker.Uint64())
			fake_name := "_dump_" + faker.Word()
			fake_event := event.New(fake_name, fake_parameters)

			fake_block_header := blockchain.BlockHeader{}
			faker.Struct(&fake_block_header)
			fake_event.BlockHeader = fake_block_header

			fake_event.Index = uint(faker.Uint32())
			fake_event.SmartcontractKey = sm.SmartcontractKey

			fake_transaction_key := blockchain.TransactionKey{}
			faker.Struct(&fake_transaction_key)
			fake_event.TransactionKey = fake_transaction_key

			logger.Info("fake event", "sm", sm.SmartcontractKey, "i", i, "data", fake_event)

			err := event.Save(db_con, fake_event)
			if err != nil {
				logger.Fatal("failed to set the data in the database", err)
			}
		}
	}

	logger.Info("finished!")
}
