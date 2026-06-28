package service

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/noPerfection/service/package_url"
	"github.com/noPerfection/topology/config"
	"github.com/stretchr/testify/require"
)

func example009HostModuleURL(t *testing.T) string {
	t.Helper()
	goModDir, err := filepath.Abs(filepath.Join("examples", "009-inproc-services"))
	require.NoError(t, err)

	mainModule := "github.com/noPerfection/service/examples/009-inproc-services/cmd/service"
	mainPackage := "github.com/noPerfection/service/examples/009-inproc-services"
	return fmt.Sprintf(
		"pkg:golang/%s#%s?root=%s&main=true",
		mainPackage,
		strings.ReplaceAll(mainModule, mainPackage, ""),
		goModDir,
	)
}

func TestIsInprocIncludedInMain_NotImported(t *testing.T) {
	hostURL := example009HostModuleURL(t)

	goModDir, err := filepath.Abs(filepath.Join("examples", "009-inproc-services"))
	require.NoError(t, err)
	inprocPkg, err := package_url.New(fmt.Sprintf(
		"pkg:golang/github.com/noPerfection/service/examples/009-inproc-services#/services/proxy?root=%s",
		goModDir,
	))
	require.NoError(t, err)

	err = IsInprocIncludedInMain(hostURL, inprocPkg)
	require.ErrorIs(t, err, ErrNotImported)
}

func TestIsInprocIncludedInMain_Imported(t *testing.T) {
	goModDir, err := filepath.Abs(filepath.Join("examples", "009-inproc-services"))
	require.NoError(t, err)

	mainModule := "github.com/noPerfection/service/examples/009-inproc-services/cmd/proxy"
	mainPackage := "github.com/noPerfection/service/examples/009-inproc-services"
	hostURL := fmt.Sprintf(
		"pkg:golang/%s#%s?root=%s&main=true",
		mainPackage,
		strings.ReplaceAll(mainModule, mainPackage, ""),
		goModDir,
	)

	inprocPkg, err := package_url.New(fmt.Sprintf(
		"pkg:golang/github.com/noPerfection/service/examples/009-inproc-services#/services/proxy?root=%s",
		goModDir,
	))
	require.NoError(t, err)

	err = IsInprocIncludedInMain(hostURL, inprocPkg)
	require.NoError(t, err)
}

func TestGetHardcodedModuleURL_009Proxy(t *testing.T) {
	hostURL := example009HostModuleURL(t)

	moduleURL, err := GetHardcodedModuleURL(hostURL, "default-name-proxy")
	require.NoError(t, err)
	require.Equal(t, "pkg:golang/github.com/noPerfection/service/examples/009-inproc-services#cmd/proxy", moduleURL)
}

func TestGetHardcodedModuleURL_009Entrypoint(t *testing.T) {
	hostURL := example009HostModuleURL(t)

	moduleURL, err := GetHardcodedModuleURL(hostURL, "entrypoint")
	require.NoError(t, err)
	require.Equal(t, "pkg:golang/github.com/noPerfection/service/examples/009-inproc-services#cmd/entrypoint", moduleURL)
}

func TestGetHardcodedModuleURL_NoMatch(t *testing.T) {
	hostURL := example009HostModuleURL(t)

	_, err := GetHardcodedModuleURL(hostURL, "missing-service")
	require.ErrorIs(t, err, ErrNoModuleURL)
}

func TestGetHardcodedModuleURL_Dynamic(t *testing.T) {
	host, err := loadHostPackageFromSource(t, `package main

import "github.com/noPerfection/service"

func main() {
	app, err := service.New("hello-world")
	if err != nil {
		panic(err)
	}
	_ = app.SetServiceConfig(service.Config{
		Name:      "dynamic",
		ModuleUrl: os.Getenv("MODULE_URL"),
	})
}
`)
	require.NoError(t, err)

	lit, err := findSetServiceConfigLiteral(host, "dynamic")
	require.NoError(t, err)
	_, err = structStringField(lit, "ModuleUrl", host.consts)
	require.ErrorIs(t, err, ErrDynamicModuleURL)
}

func TestGetHardcodedModuleURL_DynamicHelperWithArgs(t *testing.T) {
	host, err := loadHostPackageFromSource(t, `package main

import "github.com/noPerfection/service"

func main() {
	app, _ := service.New("hello-world")
	_ = app.SetServiceConfig(proxyConfig("name", "pkg:url", 8001))
}

func proxyConfig(name string, moduleURL string, port uint64) service.Config {
	return service.Config{Name: name, ModuleUrl: moduleURL}
}
`)
	require.NoError(t, err)

	_, err = findSetServiceConfigLiteral(host, "name")
	require.ErrorIs(t, err, ErrNoModuleURL)
}

func TestGetHardcodedModuleURL_Helper004(t *testing.T) {
	host, err := loadHostPackageFromSource(t, `package main

import (
	"github.com/noPerfection/service"
	topologyConfig "github.com/noPerfection/topology/config"
)

const proxyName = "default-name-proxy"

func main() {
	app, _ := service.New("hello-world")
	_ = app.SetServiceConfig(defaultNameProxyConfig())
}

func defaultNameProxyConfig() topologyConfig.Service {
	return topologyConfig.Service{
		Name:      proxyName,
		ModuleUrl: "github.com/noPerfection/service/examples/004-default-name-proxy/cmd/proxy",
	}
}
`)
	require.NoError(t, err)

	lit, err := findSetServiceConfigLiteral(host, "default-name-proxy")
	require.NoError(t, err)
	moduleURL, err := structStringField(lit, "ModuleUrl", host.consts)
	require.NoError(t, err)
	require.Equal(t, "github.com/noPerfection/service/examples/004-default-name-proxy/cmd/proxy", moduleURL)
}

func TestSetHardcodedModuleURL_UpdatesConst(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/testhost\n\ngo 1.25\n"), 0o644))
	mainDir := filepath.Join(dir, "cmd", "main")
	require.NoError(t, os.MkdirAll(mainDir, 0o755))
	mainPath := filepath.Join(mainDir, "main.go")
	require.NoError(t, os.WriteFile(mainPath, []byte(`package main

import "github.com/noPerfection/service"

const (
	proxyName      = "default-name-proxy"
	proxyModuleUrl = "old-module-url"
)

func main() {
	app, _ := service.New("hello-world")
	_ = app.SetServiceConfig(service.Config{
		Name:      proxyName,
		ModuleUrl: proxyModuleUrl,
	})
}
`), 0o644))

	hostURL := fmt.Sprintf("pkg:golang/example.com/testhost#/cmd/main?root=%s&main=true", dir)
	hostPkg, err := package_url.New(hostURL)
	require.NoError(t, err)
	libInfo := hostPkg.NewModule("services/proxy", filepath.Join(dir, "services/proxy/service.go"))

	require.NoError(t, SetHardcodedModuleURL(hostURL, "default-name-proxy", libInfo))

	got, err := GetHardcodedModuleURL(hostURL, "default-name-proxy")
	require.NoError(t, err)
	require.Equal(t, libInfo.String(), got)
}

func loadHostPackageFromSource(t *testing.T, source string) (*hostPackageAST, error) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", source, 0)
	if err != nil {
		return nil, err
	}
	files := []*ast.File{file}
	return &hostPackageAST{
		fset:   fset,
		files:  files,
		consts: packageConsts(files),
		funcs:  packageFuncs(files),
	}, nil
}

func TestUpdateInprocTopology_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/inproc-host\n\ngo 1.25\n"), 0o644))
	mainDir := filepath.Join(dir, "cmd", "host")
	require.NoError(t, os.MkdirAll(mainDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "main.go"), []byte(`package main

import "github.com/noPerfection/service"

const proxyName = "default-name-proxy"

func main() {
	app, _ := service.New("hello-world")
	_ = app.SetServiceConfig(service.Config{
		Name:      proxyName,
		ModuleUrl: "pkg:golang/example.com/inproc-host#services/proxy?root=`+dir+`",
	})
}
`), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "services", "proxy"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "services", "proxy", "service.go"), []byte(`package proxy

type Service struct{}

func New() (*Service, error) { return &Service{}, nil }
`), 0o644))

	hostURL := fmt.Sprintf("pkg:golang/example.com/inproc-host#/cmd/host?root=%s&main=true", dir)
	services := []config.Service{{
		Name:      "default-name-proxy",
		ModuleUrl: fmt.Sprintf("pkg:golang/example.com/inproc-host#services/proxy?root=%s", dir),
	}}

	require.NoError(t, UpdateInprocTopology(hostURL, services))

	topologyPath := filepath.Join(mainDir, "inproc_topology.go")
	content, err := os.ReadFile(topologyPath)
	require.NoError(t, err)
	got := string(content)
	require.Contains(t, got, `"example.com/inproc-host/services/proxy"`)
	require.Contains(t, got, "proxy1, err := proxy.New()")
	require.Contains(t, got, "inprocTopology.SetService(proxyName, proxy1)")
	require.Contains(t, got, "return inprocTopology.Start()")
}

func TestUpdateInprocTopology_Idempotent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/inproc-host\n\ngo 1.25\n"), 0o644))
	mainDir := filepath.Join(dir, "cmd", "host")
	require.NoError(t, os.MkdirAll(mainDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "main.go"), []byte(`package main

import "github.com/noPerfection/service"

const proxyName = "default-name-proxy"

func main() {
	app, _ := service.New("hello-world")
	_ = app.SetServiceConfig(service.Config{
		Name:      proxyName,
		ModuleUrl: "pkg:golang/example.com/inproc-host#services/proxy?root=`+dir+`",
	})
}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "inproc_topology.go"), []byte(`package main

import (
	"github.com/noPerfection/service"
	"example.com/inproc-host/services/proxy"
)

func startInprocTopology() error {
	inprocTopology, err := service.NewInprocExtension()
	if err != nil {
		return err
	}
	proxy1, err := proxy.New()
	if err != nil {
		return err
	}
	if err := inprocTopology.SetService(proxyName, proxy1); err != nil {
		return err
	}
	return inprocTopology.Start()
}
`), 0o644))

	hostURL := fmt.Sprintf("pkg:golang/example.com/inproc-host#/cmd/host?root=%s&main=true", dir)
	services := []config.Service{{
		Name:      "default-name-proxy",
		ModuleUrl: fmt.Sprintf("pkg:golang/example.com/inproc-host#services/proxy?root=%s", dir),
	}}

	require.NoError(t, UpdateInprocTopology(hostURL, services))
	before, err := os.ReadFile(filepath.Join(mainDir, "inproc_topology.go"))
	require.NoError(t, err)
	require.NoError(t, UpdateInprocTopology(hostURL, services))
	after, err := os.ReadFile(filepath.Join(mainDir, "inproc_topology.go"))
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))
}

func TestEnsureStartInprocTopologyCall_InsertsBeforeStart(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/inproc-host\n\ngo 1.25\n"), 0o644))
	mainDir := filepath.Join(dir, "cmd", "host")
	require.NoError(t, os.MkdirAll(mainDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "main.go"), []byte(`package main

import "github.com/noPerfection/service"

const serviceName = "hello-world"

func main() {
	app, err := service.New(serviceName)
	if err != nil {
		panic(err)
	}
	app.Route("hello", func(req service.RequestInterface) service.ReplyInterface {
		return req.Ok(nil)
	})
	if err := app.Start(); err != nil {
		panic(err)
	}
}
`), 0o644))

	hostURL := fmt.Sprintf("pkg:golang/example.com/inproc-host#/cmd/host?root=%s&main=true", dir)
	edited, err := EnsureStartInprocTopologyCall(hostURL, "hello-world")
	require.NoError(t, err)
	require.True(t, edited)

	content, err := os.ReadFile(filepath.Join(mainDir, "main.go"))
	require.NoError(t, err)
	got := string(content)
	require.Contains(t, got, "if err := startInprocTopology(); err != nil")
	require.True(t, strings.Index(got, "startInprocTopology()") < strings.Index(got, "app.Start()"))
}

func TestHostMainSourceContains_IgnoresCommentedCall(t *testing.T) {
	dir := t.TempDir()
	mainDir := filepath.Join(dir, "cmd", "host")
	require.NoError(t, os.MkdirAll(mainDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "main.go"), []byte(`package main

func main() {
	// if err := startInprocTopology(); err != nil {
	// 	panic(err)
	// }
}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/inproc-host\n\ngo 1.25\n"), 0o644))

	hostURL := fmt.Sprintf("pkg:golang/example.com/inproc-host#/cmd/host?root=%s&main=true", dir)
	contains, err := HostMainSourceContains(hostURL, startInprocTopologyCall)
	require.NoError(t, err)
	require.False(t, contains)
}

func TestEnsureStartInprocTopologyCall_AllowsEmptyNewForMain(t *testing.T) {
	dir := t.TempDir()
	mainDir := filepath.Join(dir, "cmd", "host")
	require.NoError(t, os.MkdirAll(mainDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "main.go"), []byte(`package main

import "github.com/noPerfection/service"

func main() {
	app, err := service.New()
	if err != nil {
		panic(err)
	}
	if err := app.Start(); err != nil {
		panic(err)
	}
}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/inproc-host\n\ngo 1.25\n"), 0o644))

	hostURL := fmt.Sprintf("pkg:golang/example.com/inproc-host#/cmd/host?root=%s&main=true", dir)
	edited, err := EnsureStartInprocTopologyCall(hostURL, "main")
	require.NoError(t, err)
	require.True(t, edited)
}

func TestHostMainSourceContains_IgnoresInprocTopologyDefinition(t *testing.T) {
	dir := t.TempDir()
	mainDir := filepath.Join(dir, "cmd", "host")
	require.NoError(t, os.MkdirAll(mainDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "main.go"), []byte(`package main

import "github.com/noPerfection/service"

const serviceName = "hello-world"

func main() {
	app, err := service.New(serviceName)
	if err != nil {
		panic(err)
	}
	if err := app.Start(); err != nil {
		panic(err)
	}
}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "inproc_topology.go"), []byte(`package main

func startInprocTopology() error {
	return nil
}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/inproc-host\n\ngo 1.25\n"), 0o644))

	hostURL := fmt.Sprintf("pkg:golang/example.com/inproc-host#/cmd/host?root=%s&main=true", dir)
	contains, err := HostMainSourceContains(hostURL, startInprocTopologyCall)
	require.NoError(t, err)
	require.False(t, contains)
}

func TestEnsureStartInprocTopologyCall_PreservesComments(t *testing.T) {
	dir := t.TempDir()
	mainDir := filepath.Join(dir, "cmd", "host")
	require.NoError(t, os.MkdirAll(mainDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "main.go"), []byte(`package main

import "github.com/noPerfection/service"

const serviceName = "hello-world"

func main() {
	// keep this note
	app, err := service.New(serviceName)
	if err != nil {
		panic(err)
	}
	// if err := startInprocTopology(); err != nil {
	// 	panic(err)
	// }
	if err := app.Start(); err != nil {
		panic(err)
	}
}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/inproc-host\n\ngo 1.25\n"), 0o644))

	hostURL := fmt.Sprintf("pkg:golang/example.com/inproc-host#/cmd/host?root=%s&main=true", dir)
	edited, err := EnsureStartInprocTopologyCall(hostURL, "hello-world")
	require.NoError(t, err)
	require.True(t, edited)

	content, err := os.ReadFile(filepath.Join(mainDir, "main.go"))
	require.NoError(t, err)
	got := string(content)
	require.Contains(t, got, "keep this note")
	require.Contains(t, got, "// if err := startInprocTopology(); err != nil {")
	require.Contains(t, got, "if err := startInprocTopology(); err != nil {")
}
