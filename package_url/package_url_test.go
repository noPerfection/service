package package_url

import (
	"fmt"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var readBuildInfo = debug.ReadBuildInfo

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

	require.EqualError(t, err, trimpathFlaggedError)
}

func TestNewResolves009MainPackage(t *testing.T) {
	goModDir, err := filepath.Abs(filepath.Join("..", "examples", "009-inproc-services"))
	require.NoError(t, err)

	mainModule := "github.com/noPerfection/service/examples/009-inproc-services/cmd/service"
	mainPackage := "github.com/noPerfection/service/examples/009-inproc-services"
	mushroomURL := fmt.Sprintf("pkg:golang/%s#%s?root=%s&main=true", mainPackage, strings.ReplaceAll(mainModule, mainPackage, ""), goModDir)

	info, err := New(mushroomURL)
	require.NoError(t, err)

	require.Equal(t, mainModule, info.module)
	require.Equal(t, mainPackage, info.pkg)
	require.Equal(t, goModDir, info.pkgDir)
	require.Equal(t, filepath.Join(goModDir, "cmd", "service"), info.moduleDir)
	require.True(t, info.mainModule)
	require.Equal(t, goModDir, info.mushroomHypha.AdditionalProps["root"])
}

func TestNewRejectsSymbolicURL(t *testing.T) {
	_, err := New("hello-world")
	require.Error(t, err)
	require.Contains(t, err.Error(), "symbolic")
}
