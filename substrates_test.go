package service

import (
	"testing"

	"github.com/ahmetson/mushroom"
	"github.com/stretchr/testify/require"
)

type stubSubstrate struct {
	pattern string
}

func (s stubSubstrate) MushroomURL() string { return s.pattern }

func (s stubSubstrate) Forage(url mushroom.Hypha) (any, error) { return nil, nil }

func (s stubSubstrate) Digest(url mushroom.Hypha, data any, soil *mushroom.Soil) (mushroom.Mycelium, error) {
	return nil, nil
}

func (s stubSubstrate) Sow(url mushroom.Hypha, data any) error { return nil }

func TestSubstratesForTopologyIncludesOSSubstrate(t *testing.T) {
	merged := substratesForTopology()
	require.NotEmpty(t, merged)
	require.Equal(t, "pkg:os$#$", merged[len(merged)-1].MushroomURL())
}

func TestRegisterBuiltinSubstrate(t *testing.T) {
	original := builtinSubstrates
	t.Cleanup(func() {
		builtinSubstrates = original
	})

	require.Error(t, RegisterBuiltinSubstrate(nil))

	substrate := stubSubstrate{pattern: "pkg:stub/$#$.dat"}
	require.NoError(t, RegisterBuiltinSubstrate(substrate))
	require.Len(t, builtinSubstrates, len(original)+1)

	merged := substratesForTopology()
	require.GreaterOrEqual(t, len(merged), 2)
	require.Equal(t, substrate, merged[len(merged)-2])
	require.Equal(t, "pkg:os$#$", merged[len(merged)-1].MushroomURL())
}
