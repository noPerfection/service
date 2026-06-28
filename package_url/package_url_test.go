package package_url

import (
	"fmt"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/ahmetson/mushroom"
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

func TestServiceNameToPackageName(t *testing.T) {
	require.Equal(t, "hello_world", ServiceNameToPackageName("  hello-world  "))
	require.Equal(t, "default_name_proxy", ServiceNameToPackageName("default-name-proxy"))
	require.Equal(t, "hello_world", ServiceNameToPackageName("hello   world"))
}

func TestImportClause(t *testing.T) {
	rootPkg := &PackageInfo{
		mushroomHypha: mushroom.Hypha{
			URL:       true,
			Type:      "golang",
			PackageID: "github.com/noPerfection/service",
		},
	}
	require.Equal(t, "github.com/noPerfection/service", rootPkg.ImportClause())

	subPkg := &PackageInfo{
		mushroomHypha: mushroom.Hypha{
			URL:       true,
			Type:      "golang",
			PackageID: "github.com/noPerfection/service/examples/009-inproc-services",
			ModuleID:  "cmd/service",
		},
	}
	require.Equal(t, "github.com/noPerfection/service/examples/009-inproc-services/cmd/service", subPkg.ImportClause())

	servicesPkg := &PackageInfo{
		mushroomHypha: mushroom.Hypha{
			URL:       true,
			Type:      "golang",
			PackageID: "github.com/noPerfection/service/examples/009-inproc-services",
			ModuleID:  "services/default_name_proxy",
		},
	}
	require.Equal(t, "github.com/noPerfection/service/examples/009-inproc-services/services/default_name_proxy", servicesPkg.ImportClause())
}

func TestIsFileExistMissingFile(t *testing.T) {
	goModDir, err := filepath.Abs(filepath.Join("..", "examples", "009-inproc-services"))
	require.NoError(t, err)

	mainModule := "github.com/noPerfection/service/examples/009-inproc-services/cmd/service"
	mainPackage := "github.com/noPerfection/service/examples/009-inproc-services"
	mushroomURL := fmt.Sprintf("pkg:golang/%s#%s?root=%s&main=true", mainPackage, strings.ReplaceAll(mainModule, mainPackage, ""), goModDir)

	_, err = IsFileExist(mushroomURL, "inproc_topology.go")
	require.Error(t, err)
	require.Contains(t, err.Error(), "doesn't exist")
}

func TestNewResolvesThirdPartyModuleWithReplace(t *testing.T) {
	goModDir, err := filepath.Abs(filepath.Join("..", "examples", "009-inproc-services"))
	require.NoError(t, err)

	mushroomURL := fmt.Sprintf("pkg:golang/github.com/noPerfection/service?root=%s", goModDir)
	info, err := New(mushroomURL)
	require.NoError(t, err)

	require.True(t, info.IsThirdParty())
	require.True(t, info.IsEditable())
	require.Equal(t, "true", info.mushroomHypha.AdditionalProps[thirdPartyProp])
	require.Contains(t, info.String(), "thirdparty=true")
	require.NotEmpty(t, info.SourceFiles())
	require.Equal(t, "github.com/noPerfection/service", info.pkg)
}

func TestNewThirdPartyWithoutReplaceIsNotEditable(t *testing.T) {
	goMod := []byte(`module example.com/app

go 1.25

require github.com/noPerfection/service v0.0.0
`)
	require.Empty(t, parseReplaces(goMod, t.TempDir())["github.com/noPerfection/service"])

	required, ok := findRequiredModule("github.com/noPerfection/service", parseRequires(goMod))
	require.True(t, ok)
	require.Equal(t, "github.com/noPerfection/service", required)
}

func TestEnsureEditableThirdPartyWithoutReplace(t *testing.T) {
	hypha := mushroom.Hypha{
		URL:       true,
		Type:      "golang",
		PackageID: "github.com/noPerfection/service",
		AdditionalProps: map[string]string{
			thirdPartyProp: "true",
		},
	}
	info := newThirdPartyInfo(hypha, t.TempDir(), "github.com/noPerfection/service", "github.com/noPerfection/service")

	require.False(t, info.IsEditable())
	err := info.EnsureEditable()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrThirdPartyNotEditable)
}
