package abi

import (
	"fmt"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/db/handler"
)

// Save the ABI in the Database
func SetInDatabase(db *remote.ClientSocket, a *Abi) error {
	request := handler.DatabaseQueryRequest{
		Fields:    []string{"abi_id", "body"},
		Tables:    []string{"static_abi"},
		Arguments: []interface{}{a.Id, a.Bytes},
	}
	var reply handler.InsertReply

	err := handler.INSERT.Request(db, request, &reply)
	if err != nil {
		return fmt.Errorf("handler.WRITE.Push: %w", err)
	}
	return nil
}

// Get all abis from database
func GetAllFromDatabase(db *remote.ClientSocket) ([]*Abi, error) {
	request := handler.DatabaseQueryRequest{
		Fields:    []string{"abi_id", "body"},
		Tables:    []string{"static_abi"},
		Arguments: []interface{}{},
	}
	var reply handler.SelectAllReply

	err := handler.SELECT_ALL.Request(db, request, &reply)
	if err != nil {
		return nil, fmt.Errorf("handler.WRITE.Push: %w", err)
	}

	abis := make([]*Abi, len(reply.Rows))

	// Loop through rows, using Scan to assign column data to struct fields.
	for i, raw := range reply.Rows {
		abi, err := New(raw)
		if err != nil {
			return nil, fmt.Errorf("New Abi from database result: %w", err)
		}
		abis[i] = abi
	}
	return abis, err
}
