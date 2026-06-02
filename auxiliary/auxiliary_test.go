package auxiliary

import (
	"github.com/noPerfection/datatype/data_type/key_value"
	"github.com/noPerfection/datatype/message"
	"github.com/noPerfection/log"
	"github.com/noPerfection/os/arg"
	clientConfig "github.com/noPerfection/protocol/client/config"
	"github.com/noPerfection/protocol/handler/base"
	handlerConfig "github.com/noPerfection/protocol/handler/config"
	"github.com/noPerfection/protocol/handler/route"
	"github.com/noPerfection/protocol/handler/sync_replier"
	serviceLib "github.com/noPerfection/service"
	"github.com/pebbe/zmq4"
	"github.com/stretchr/testify/suite"
	win "os"
	"testing"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing orchestra
type TestAuxiliarySuite struct {
	suite.Suite

	parent  *serviceLib.Independent // the manager to test
	url     string                  // dependency source code
	name    string                  // the name of the dependency
	handler base.Interface
	logger  *log.Logger

	defaultHandleFunc route.HandleFunc0
	cmd1              string
	handlerCategory   string
}

func (test *TestAuxiliarySuite) SetupTest() {
	s := test.Suite.Require

	// A valid source code that we want to download
	test.url = "github.com/ahmetson/parent-lib"
	test.name = "service_1"

	// handler
	syncReplier := sync_replier.New()
	test.defaultHandleFunc = func(req message.RequestInterface) message.ReplyInterface {
		return req.Ok(key_value.New())
	}
	test.cmd1 = "hello"
	s().NoError(syncReplier.Route(test.cmd1, test.defaultHandleFunc))
	test.handler = syncReplier

	var err error
	test.logger, err = log.New("test", true)
	s().NoError(err)

	test.handlerCategory = "main"
	inprocConfig := handlerConfig.NewInternalHandler(handlerConfig.SyncReplierType, test.handlerCategory)
	test.handler.SetConfig(inprocConfig)
	s().NoError(test.handler.SetLogger(test.logger))
}

// Test_10_NewAuxiliary tests NewAuxiliary
func (test *TestAuxiliarySuite) Test_10_NewAuxiliary() {
	s := test.Suite.Require

	// Creating an Auxiliary must fail since no Parent flag
	_, err := NewAuxiliary()
	s().Error(err)

	// Creating an auxiliary must fail since parent is not a valid config
	win.Args = append(win.Args, arg.NewFlag(ParentFlag, "parent"))
	_, err = NewAuxiliary()
	s().Error(err)
	DeleteLastFlags(1)

	// Creating an auxiliary with the valid flags must succeed
	parentClient := clientConfig.New(test.url+"_parent", test.name+"_parent", 6000, zmq4.REP)
	parentKv, err := key_value.NewFromInterface(parentClient)
	s().NoError(err)
	parentStr := parentKv.String()
	win.Args = append(win.Args, arg.NewFlag(ParentFlag, parentStr))

	auxiliary, err := NewAuxiliary(test.name)
	s().NoError(err)

	DeleteLastFlags(1)

	s().NoError(auxiliary.Context().Close())
}

func TestAuxiliary(t *testing.T) {
	suite.Run(t, new(TestAuxiliarySuite))
}
