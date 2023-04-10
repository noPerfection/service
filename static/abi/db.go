package abi

import (
	"fmt"

	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/db/handler"
)

// Save the ABI in the Database
func SetInDatabase(db *remote.ClientSocket, a *Abi) error {
	request := handler.DatabaseQueryRequest{
		Query:     `INSERT IGNORE INTO static_abi (abi_id, body) VALUES (?, ?) `,
		Arguments: []interface{}{a.Id, a.Bytes},
	}
	var reply handler.WriteReply

	err := handler.WRITE.Request(db, request, &reply)
	if err != nil {
		return fmt.Errorf("handler.WRITE.Push: %w", err)
	}
	return nil
}

// Get all abis from database
func GetAllFromDatabase(db *remote.ClientSocket) ([]*Abi, error) {
	request := handler.DatabaseQueryRequest{
		Query:     "SELECT body, abi_id FROM static_abi",
		Arguments: []interface{}{},
		Outputs:   []interface{}{[]byte{}, ""},
	}
	var reply handler.ReadAllReply

	err := handler.WRITE.Request(db, request, &reply)
	if err != nil {
		return nil, fmt.Errorf("handler.WRITE.Push: %w", err)
	}

	abis := make([]*Abi, len(reply.Rows))

	// Loop through rows, using Scan to assign column data to struct fields.
	for i, raw := range reply.Rows {
		abi := Abi{
			Bytes: raw.Outputs[0].([]byte),
			Id:    raw.Outputs[1].(string),
		}
		abis[i] = &abi
	}
	return abis, err
}
