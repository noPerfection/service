package service

import (
	"fmt"

	"github.com/ahmetson/mushroom"
	osSubstrate "github.com/noPerfection/os/substrate"
	"github.com/noPerfection/topology"
)

var builtinSubstrates []mushroom.Substrate

// RegisterBuiltinSubstrate registers a substrate used when loading topology config.
// Built-in substrates live in the service package, not topology.
func RegisterBuiltinSubstrate(substrate mushroom.Substrate) error {
	if substrate == nil {
		return fmt.Errorf("substrate is nil")
	}
	builtinSubstrates = append(builtinSubstrates, substrate)
	return nil
}

func substratesForTopology(extra ...mushroom.Substrate) []mushroom.Substrate {
	all := make([]mushroom.Substrate, 0, 1+len(builtinSubstrates)+len(extra))
	all = append(all, builtinSubstrates...)
	all = append(all, osSubstrate.New())
	all = append(all, extra...)
	return all
}

func newTopologyHandler(configPath string, extra ...mushroom.Substrate) (*topology.Handler, error) {
	return topology.NewHandler(configPath, substratesForTopology(extra...)...)
}
