package service

import (
	"runtime/debug"
	"testing"

	"github.com/noPerfection/topology/config"
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
	service := config.Service{Name: "app"}

	require.NoError(t, fillDefaultModuleURL(&service))

	require.Equal(t, "example.com/app", service.ModuleUrl)
}

func TestFillDefaultModuleURLKeepsExplicitModuleURL(t *testing.T) {
	stubBuildInfo(t, "example.com/app", true)
	service := config.Service{
		Name:      "app",
		ModuleUrl: "example.com/explicit",
	}

	require.NoError(t, fillDefaultModuleURL(&service))

	require.Equal(t, "example.com/explicit", service.ModuleUrl)
}

func TestFillDefaultModuleURLRequiresBuildInfo(t *testing.T) {
	stubBuildInfo(t, "", false)
	service := config.Service{Name: "app"}

	err := fillDefaultModuleURL(&service)

	require.EqualError(t, err, moduleURLBuildInfoError)
}
