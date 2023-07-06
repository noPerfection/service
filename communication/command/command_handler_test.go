package command

import (
	"testing"

	"github.com/Seascape-Foundation/sds-service-lib/communication/message"
	"github.com/Seascape-Foundation/sds-service-lib/log"
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
	handlers := EmptyHandlers()
	suite.Len(handlers, 0)

	command1 := New("command_1")
	command1Handler := func(request message.Request, _ log.Logger, _ ...interface{}) message.Reply {
		return message.Reply{
			Status:     message.OK,
			Message:    "",
			Parameters: request.Parameters.Set("id", uint64(1)),
		}
	}
	command2 := New("command_2")
	command2Handler := func(request message.Request, _ log.Logger, _ ...interface{}) message.Reply {
		return message.Reply{
			Status:     message.OK,
			Message:    "",
			Parameters: request.Parameters.Set("id", uint64(2)),
		}
	}
	handlers = handlers.Add(command1, command1Handler)
	suite.Len(handlers, 1)
	suite.True(handlers.Exist(command1))
	suite.False(handlers.Exist(command2))

	handlers = handlers.Add(command2, command2Handler)
	suite.Len(handlers, 2)
	suite.True(handlers.Exist(command1))
	suite.True(handlers.Exist(command2))

	commandNames := handlers.CommandNames()
	suite.Equal(len(handlers), len(commandNames))
	commandNameStrings := []string{
		command1.String(),
		command2.String(),
	}
	suite.EqualValues(commandNames, commandNameStrings)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCommandHandlers(t *testing.T) {
	suite.Run(t, new(TestCommandHandler))
}
