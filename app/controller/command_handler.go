package controller

import (
	"github.com/blocklords/gosds/app/remote/message"
	"github.com/blocklords/gosds/db"
)

type CommandHandlers map[string]func(*db.Database, message.Request) message.Reply
