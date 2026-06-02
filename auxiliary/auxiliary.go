package auxiliary

import (
	"fmt"
	"github.com/noPerfection/datatype/data_type/key_value"
	"github.com/noPerfection/os/arg"
	clientConfig "github.com/noPerfection/protocol/client/config"
	serviceLib "github.com/noPerfection/service"
	"github.com/noPerfection/service/manager"
)

const ParentFlag = "parent"

type Auxiliary struct {
	*serviceLib.Service
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
	parentKv, err := key_value.NewFromString(parentStr)
	if err != nil {
		return nil, fmt.Errorf("key_value.NewFromString('%s'): %w", ParentFlag, err)
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

	independent, err := serviceLib.New(name...)
	if err != nil {
		return nil, fmt.Errorf("new independent parent: %w", err)
	}

	return &Auxiliary{Service: independent, ParentManager: parent, ParentConfig: &parentConfig}, nil
}
