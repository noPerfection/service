package controller

import (
	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/db"
	"github.com/charmbracelet/log"
)

// command name => function
type CommandHandlers map[string]func(*db.Database, message.Request, log.Logger) message.Reply

// Check does command handler exist
func (c CommandHandlers) Exist(command string) bool {
	_, ok := c[command]
	return ok
}
