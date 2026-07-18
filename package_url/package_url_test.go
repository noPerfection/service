package package_url

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/ahmetson/mushroom"
	"github.com/stretchr/testify/require"
)

func example009GoModDir(t *testing.T) string {
	t.Helper()
	t.Setenv("GOWORK", "off")

	goModDir, err := filepath.Abs(filepath.Join("..", "examples", "009-inproc-services"))
	require.NoError(t, err)

	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(goModDir))
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	return goModDir
}

func TestFillDefaultModuleURLUsesMainModulePath(t *testing.T) {
	buildInfo, ok := debug.ReadBuildInfo()
	require.True(t, ok)

	dir := t.TempDir()
	mainFile := filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(mainFile, []byte("package main\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fallback.example/app\n\ngo 1.25\n"), 0o644))

	_, mainPackage, err := fileToModuleAndPackage(mainFile)
	require.NoError(t, err)
	require.Equal(t, buildInfo.Main.Path, mainPackage)
}

func TestFillDefaultModuleURLRequiresBuildInfo(t *testing.T) {
	_, err := FillDefaultModuleURL()

	require.EqualError(t, err, trimpathFlaggedError)
}

func TestNewResolves009MainPackage(t *testing.T) {
	goModDir := example009GoModDir(t)

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
	goModDir := example009GoModDir(t)

	mainModule := "github.com/noPerfection/service/examples/009-inproc-services/cmd/service"
	mainPackage := "github.com/noPerfection/service/examples/009-inproc-services"
	mushroomURL := fmt.Sprintf("pkg:golang/%s#%s?root=%s&main=true", mainPackage, strings.ReplaceAll(mainModule, mainPackage, ""), goModDir)

	_, err := IsFileExist(mushroomURL, "missing_file.go")
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
