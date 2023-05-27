package command

import (
	"testing"

	"github.com/blocklords/sds/app/log"
	"github.com/blocklords/sds/app/communication/message"
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

	command_1 := New("command_1")
	command_1_handler := func(request message.Request, _ log.Logger, _ ...interface{}) message.Reply {
		return message.Reply{
			Status:     message.OK,
			Message:    "",
			Parameters: request.Parameters.Set("id", uint64(1)),
		}
	}
	command_2 := New("command_2")
	command_2_handler := func(request message.Request, _ log.Logger, _ ...interface{}) message.Reply {
		return message.Reply{
			Status:     message.OK,
			Message:    "",
			Parameters: request.Parameters.Set("id", uint64(2)),
		}
	}
	handlers = handlers.Add(command_1, command_1_handler)
	suite.Len(handlers, 1)
	suite.True(handlers.Exist(command_1))
	suite.False(handlers.Exist(command_2))

	handlers = handlers.Add(command_2, command_2_handler)
	suite.Len(handlers, 2)
	suite.True(handlers.Exist(command_1))
	suite.True(handlers.Exist(command_2))

	command_names := handlers.CommandNames()
	suite.Equal(len(handlers), len(command_names))
	command_name_strings := []string{
		command_1.String(),
		command_2.String(),
	}
	suite.EqualValues(command_names, command_name_strings)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCommandHandlers(t *testing.T) {
	suite.Run(t, new(TestCommandHandler))
}
