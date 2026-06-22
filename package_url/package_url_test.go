package package_url

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/require"
)

func stubBuildInfo(t *testing.T, moduleURL string, ok bool) {
	t.Helper()

	original := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		if !ok {
			return nil, false
		}
		return &debug.BuildInfo{
			Main: debug.Module{Path: moduleURL},
		}, true
	}
	t.Cleanup(func() {
		readBuildInfo = original
	})
}

func TestFillDefaultModuleURLUsesMainModulePath(t *testing.T) {
	stubBuildInfo(t, "example.com/app", true)

	moduleURL, err := FillDefaultModuleURL()
	require.NoError(t, err)

	require.Equal(t, "example.com/app", moduleURL)
}

func TestFillDefaultModuleURLRequiresBuildInfo(t *testing.T) {
	stubBuildInfo(t, "", false)

	_, err := FillDefaultModuleURL()

	require.EqualError(t, err, moduleURLBuildInfoError)
}
