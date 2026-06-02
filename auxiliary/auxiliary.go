package auxiliary

import (
	"fmt"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/os/arg"
	clientConfig "github.com/noPerfection/protocol/client/config"
	serviceLib "github.com/noPerfection/service"
	"github.com/noPerfection/service/manager"
)

const ParentFlag = "parent"

type Auxiliary struct {
	*serviceLib.Independent
	ParentManager *manager.Client // parent to work with
	ParentConfig  *clientConfig.Client
}

// NewAuxiliary creates a parent with the parent.
// It requires a parent flag
func NewAuxiliary(name ...string) (*Auxiliary, error) {
	if !arg.FlagExist(ParentFlag) {
		return nil, fmt.Errorf("missing %s flag", arg.NewFlag(ParentFlag))
	}

	//
	// Parent config in a raw string format
	//
	parentStr := arg.FlagValue(ParentFlag)
	parentKv, err := datatype.NewFromString(parentStr)
	if err != nil {
		return nil, fmt.Errorf("datatype.NewFromString('%s'): %w", ParentFlag, err)
	}

	//
	// Parent config
	//
	var parentConfig clientConfig.Client
	err = parentKv.Interface(&parentConfig)
	if err != nil {
		return nil, fmt.Errorf("parentKv.Interface: %w", err)
	}
	if len(parentConfig.Id) == 0 {
		return nil, fmt.Errorf("empty parent")
	}
	parentConfig.UrlFunc(clientConfig.Url)

	//
	// Parent client
	//
	parent, err := manager.NewClient(&parentConfig)
	if err != nil {
		return nil, fmt.Errorf("manager.NewClient('parentConfig'): %w", err)
	}

	params := make([]interface{}, len(name))
	for i, value := range name {
		params[i] = value
	}

	independent, err := serviceLib.New(params...)
	if err != nil {
		return nil, fmt.Errorf("new independent parent: %w", err)
	}

	return &Auxiliary{Independent: independent, ParentManager: parent, ParentConfig: &parentConfig}, nil
}
