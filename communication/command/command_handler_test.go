package command

import (
	"github.com/ahmetson/service-lib/remote"
	"testing"

	"github.com/ahmetson/service-lib/communication/message"
	"github.com/ahmetson/service-lib/log"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestCommandHandler struct {
	suite.Suite
}

// Make sure that Account is set to five
// before each test
func (suite *TestCommandHandler) SetupTest() {
	handlers := NewRoutes()
	suite.Len(handlers, 0)

	command1 := "command_1"
	command1Handler := func(request message.Request, _ log.Logger, _ remote.Clients) message.Reply {
		return message.Reply{
			Status:     message.OK,
			Message:    "",
			Parameters: request.Parameters.Set("id", uint64(1)),
		}
	}
	command2 := "command_2"
	command2Handler := func(request message.Request, _ log.Logger, _ remote.Clients) message.Reply {
		return message.Reply{
			Status:     message.OK,
			Message:    "",
			Parameters: request.Parameters.Set("id", uint64(2)),
		}
	}
	handlers.Add(command1, command1Handler)
	suite.Equal(handlers.Len(), 1)
	suite.True(handlers.Exist(command1))
	suite.False(handlers.Exist(command2))

	handlers.Add(command2, command2Handler)
	suite.Equal(handlers.Len(), 2)
	suite.True(handlers.Exist(command1))
	suite.True(handlers.Exist(command2))

	commandNames := Commands(handlers)
	suite.Equal(handlers.Len(), len(commandNames))
	commandNameStrings := []string{
		command1,
		command2,
	}
	suite.EqualValues(commandNames, commandNameStrings)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCommandHandlers(t *testing.T) {
	suite.Run(t, new(TestCommandHandler))
}
