package service

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/noPerfection/datatype"
	clientSyncReplier "github.com/noPerfection/protocol/client/sync_replier"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/manager"
	"github.com/noPerfection/service/package_url"
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
)

const (
	InprocTopologyServiceName  = "inproc-topology"
	inprocTopologyProbeTimeout = 50 * time.Millisecond
)

var (
	DefaultInprocTopologyEndpoint        = message.NewEndpoint(InprocTopologyServiceName, 0)
	DefaultInprocTopologyManagerEndpoint = message.NewEndpoint(InprocTopologyServiceName+"_manager", 0)
)

// InprocTopologyService manages in-process services registered by service name.
type InprocTopologyService struct {
	*Extension
	services map[string]Service
}

// inprocTopologyExtensionServiceLink returns the mushroom URL of the inproc-topology
// extension service record in topology, e.g. pkg:$?var=services[name:inproc-topology].
func inprocTopologyExtensionServiceLink() string {
	return "pkg:$?var=services[name:" + InprocTopologyServiceName + "]"
}

func defaultInprocTopologyExtensionServiceConfig() Config {
	return Config{
		Type:      ExtensionType,
		Name:      InprocTopologyServiceName,
		ModuleUrl: "pkg:golang/github.com/noPerfection/service",
		Handlers: []Handler{
			IndependentHandler{
				Type:     SyncReplierType,
				Category: ServiceManagerCategory,
				Endpoint: DefaultInprocTopologyManagerEndpoint,
			},
			ExtensionHandler{
				IndependentHandler: IndependentHandler{
					Type:     ReplierType,
					Category: DefaultHandlerCategory,
					Endpoint: DefaultInprocTopologyEndpoint,
				},
			},
		},
	}
}

// NewInprocExtension returns an inproc topology extension service.
// Call SetTopologyParams before Start to configure the topology JSON path.
func NewInprocExtension() (*InprocTopologyService, error) {
	extension, err := NewExt(InprocTopologyServiceName)
	if err != nil {
		return nil, err
	}

	if err := extension.SetEndpoint(DefaultInprocTopologyManagerEndpoint, ServiceManagerCategory); err != nil {
		return nil, fmt.Errorf("SetEndpoint: %w", err)
	}

	inprocTopology := &InprocTopologyService{
		Extension: extension,
		services:  make(map[string]Service),
	}
	if err := inprocTopology.Route(manager.StartService, inprocTopology.onStartService); err != nil {
		return nil, fmt.Errorf("Route(%q): %w", manager.StartService, err)
	}
	return inprocTopology, nil
}

func (t *InprocTopologyService) onStartService(req RequestInterface) ReplyInterface {
	serviceName, err := req.RouteParameters().StringValue("service")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('service'): %v", err))
	}
	id, err := t.startService(serviceName)
	if err != nil {
		return req.Fail(fmt.Sprintf("startService(%q): %v", serviceName, err))
	}
	return req.Ok(KeyValue().Set("id", id))
}

// SetService registers an in-process service instance for a topology mushroom URL.
func (t *InprocTopologyService) SetService(mushroomURL string, svc Service) error {
	if t == nil {
		return fmt.Errorf("inproc topology is nil")
	}
	if svc == nil {
		return fmt.Errorf("inproc service is nil")
	}
	mushroomURL = dereferenceMushroomURL(mushroomURL)
	if mushroomURL == "" {
		return fmt.Errorf("mushroom url is empty")
	}
	tp := t.topology()
	if tp == nil {
		return fmt.Errorf("topology is nil")
	}

	serviceConfig, err := tp.Service(mushroomURL)
	if err != nil {
		return fmt.Errorf("topology.Service(%q): %w", mushroomURL, err)
	}

	t.services[serviceConfig.Name] = svc
	return nil
}

// Start starts the inproc topology extension.
func (t *InprocTopologyService) Start() error {
	if t == nil {
		return fmt.Errorf("inproc topology is nil")
	}
	return t.Extension.Start()
}

func (t *InprocTopologyService) startService(name string) (string, error) {
	svc, ok := t.services[name]
	if !ok {
		return "", fmt.Errorf("inproc service %q is not registered", name)
	}
	tp := t.topology()
	if tp == nil {
		return "", fmt.Errorf("topology is nil")
	}
	serviceConfig, err := tp.Service(name)
	if err != nil {
		return "", fmt.Errorf("topology.Service(%q): %w", name, err)
	}
	if !serviceConfig.IsInproc() {
		return "", fmt.Errorf("cannot start service %q: not inproc", serviceConfig.Name)
	}
	if err := svc.Start(); err != nil {
		return "", err
	}
	return strconv.Itoa(os.Getpid()), nil
}

var (
	// ErrNotImported is returned when the host main package does not import the inproc library.
	ErrNotImported = errors.New("inproc package not imported in host main")
	// ErrNoModuleURL is returned when no SetServiceConfig matches the target service name or ModuleUrl is absent.
	ErrNoModuleURL = errors.New("SetServiceConfig for service has no ModuleUrl")
	// ErrDynamicModuleURL is returned when ModuleUrl or Name is not a static const or literal.
	ErrDynamicModuleURL = errors.New("ModuleUrl is not a static const or literal")
	// ErrDynamicServiceName is returned when the host service name passed to New/NewExt/NewProxy is not static.
	ErrDynamicServiceName = errors.New("host service name is not a static string")
	// ErrInprocTopologyPresentNotRunning is returned when startInprocTopology appears in main but topology is not running.
	ErrInprocTopologyPresentNotRunning = errors.New("startInprocTopology is present in main but inproc topology is not running")
	// ErrNeedToRerun marks intentional staged exits (rebuild/re-run) that must not roll back topology.
	ErrNeedToRerun = errors.New("need to rerun")
)

// NeedToRerunErr is returned when source or topology was updated and the process must be rebuilt or re-run.
type NeedToRerunErr struct {
	Message string
}

func (e *NeedToRerunErr) Error() string {
	if e == nil {
		return ErrNeedToRerun.Error()
	}
	return e.Message
}

func (e *NeedToRerunErr) Is(target error) bool {
	return target == ErrNeedToRerun
}

// NeedToRerun formats an intentional rebuild/re-run exit.
func NeedToRerun(format string, args ...any) error {
	return &NeedToRerunErr{Message: fmt.Sprintf(format, args...)}
}

// IsNeedToRerunErr reports whether err is or wraps ErrNeedToRerun.
func IsNeedToRerunErr(err error) bool {
	return errors.Is(err, ErrNeedToRerun)
}

// IsInprocIncludedInMain reports whether hostModuleURL's main package imports inprocPkg.
func IsInprocIncludedInMain(hostModuleURL string, inprocPkg *package_url.PackageInfo) error {
	if inprocPkg == nil {
		return fmt.Errorf("inproc package info is nil")
	}
	importPath := inprocPkg.ImportClause()
	if importPath == "" {
		return fmt.Errorf("inproc import clause is empty")
	}

	host, err := loadHostPackage(hostModuleURL)
	if err != nil {
		return err
	}

	if _, ok := importLocalName(host.files, importPath); !ok {
		return ErrNotImported
	}
	return nil
}

// GetHardcodedModuleURL returns the ModuleUrl hardcoded in host main's SetServiceConfig for serviceName.
// It parses hostModuleURL source only (not runtime WithHardcodedTopology).
func GetHardcodedModuleURL(hostModuleURL string, serviceName string) (moduleURL string, err error) {
	if serviceName == "" {
		return "", fmt.Errorf("service name is empty")
	}

	host, err := loadHostPackage(hostModuleURL)
	if err != nil {
		return "", err
	}

	lit, err := findModuleURLStringLit(host, serviceName)
	if err != nil {
		return "", err
	}
	unquoted, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", fmt.Errorf("module url literal: %w", err)
	}
	if unquoted == "" {
		return "", ErrNoModuleURL
	}
	return unquoted, nil
}

// SetHardcodedModuleURL updates the ModuleUrl in host main's SetServiceConfig for serviceName.
// The new value is taken from libInfo.String(). Only static literals and package consts are updated.
func SetHardcodedModuleURL(hostModuleURL string, serviceName string, libInfo *package_url.PackageInfo) error {
	if serviceName == "" {
		return fmt.Errorf("service name is empty")
	}
	if libInfo == nil {
		return fmt.Errorf("lib package info is nil")
	}
	newModuleURL := libInfo.String()
	if newModuleURL == "" {
		return fmt.Errorf("lib module url is empty")
	}

	host, err := loadHostPackage(hostModuleURL)
	if err != nil {
		return err
	}

	lit, err := findModuleURLStringLit(host, serviceName)
	if err != nil {
		return err
	}
	lit.Value = strconv.Quote(newModuleURL)
	return writeHostPackage(host)
}

// MainPackageToLibraryPackage converts main package info into a library package
// at services/<name>/service.go under the same module root.
// It returns the library package info and whether that module already exists.
// This is just prepares the package_url.PackageInfo.
//
// For actual code transformation use the MainPackageToLibraryAI()
func MainPackageToLibraryPackage(pkgInfo *package_url.PackageInfo) (asLibInfo *package_url.PackageInfo, exists bool, err error) {
	if pkgInfo == nil {
		return nil, false, fmt.Errorf("package info is nil")
	}

	packageName := pkgInfo.PackageName()
	moduleID := fmt.Sprintf("services/%s", packageName)
	moduleFilename := path.Join(pkgInfo.Dir(), fmt.Sprintf("services/%s/service.go", packageName))

	exists, err = pkgInfo.IsModuleExist(moduleID)
	if err != nil {
		return nil, false, fmt.Errorf("package_url.IsModuleExist(%s): %w", moduleID, err)
	}

	return pkgInfo.NewModule(moduleID, moduleFilename), exists, nil
}

// MainPackageToLibraryAI reads main.go, asks ai to extract a service library, and writes the files.
func MainPackageToLibraryAI(ai *AiClient, mainPkg, modulePkg *package_url.PackageInfo) error {
	if ai == nil {
		return fmt.Errorf("ai client is nil")
	}
	if mainPkg == nil {
		return fmt.Errorf("main package info is nil")
	}
	if modulePkg == nil {
		return fmt.Errorf("module package info is nil")
	}

	mainFiles := mainPkg.SourceFiles()
	if len(mainFiles) == 0 {
		return fmt.Errorf("main package has no source files")
	}
	moduleFiles := modulePkg.SourceFiles()
	if len(moduleFiles) == 0 {
		return fmt.Errorf("module package has no source files")
	}

	packageName := modulePkg.PackageName()
	if packageName == "" {
		return fmt.Errorf("module package name is empty")
	}
	importClause := modulePkg.ImportClause()
	if importClause == "" {
		return fmt.Errorf("module import clause is empty")
	}

	mainGoPath := mainFiles[0]
	serviceFilePath := moduleFiles[0]

	mainGo, err := os.ReadFile(mainGoPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", mainGoPath, err)
	}

	serviceCode, updatedMain, err := ai.MainPackageToLibrary(packageName, importClause, string(mainGo))
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(serviceFilePath), 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", filepath.Dir(serviceFilePath), err)
	}
	if err := os.WriteFile(serviceFilePath, []byte(serviceCode), 0o644); err != nil {
		return fmt.Errorf("write %q: %w", serviceFilePath, err)
	}
	if err := os.WriteFile(mainGoPath, []byte(updatedMain), 0o644); err != nil {
		return fmt.Errorf("write %q: %w", mainGoPath, err)
	}
	return nil
}

// ProbeInprocServiceRunning asks the service manager whether the inproc service is running.
func ProbeInprocServiceRunning(service config.Service) (bool, error) {
	endpoint, err := managerEndpointForService(service)
	if err != nil {
		return false, err
	}

	client, err := clientSyncReplier.NewClient(endpoint.Id, endpoint.Port)
	if err != nil {
		return false, fmt.Errorf("sync_replier.NewClient: %w", err)
	}
	defer client.Close()

	client.Timeout(inprocTopologyProbeTimeout)
	client.Attempt(1)

	reply, err := client.Request(&message.Request{
		Command:    manager.IsServiceRunning,
		Parameters: datatype.New().Set("service", service.Name),
	})
	if err != nil {
		if errors.Is(err, message.RequestTimeoutError) {
			return false, nil
		}
		return false, err
	}
	if !reply.IsOK() {
		return false, fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	}

	running, err := reply.ReplyParameters().BoolValue("running")
	if err != nil {
		return false, fmt.Errorf("reply.Parameters.GetBoolean('running'): %w", err)
	}
	return running, nil
}

func managerEndpointForService(service config.Service) (message.Endpoint, error) {
	managerHandler, err := service.HandlerByCategory(topology.ServiceManagerCategory)
	if err == nil {
		handler, ok := managerHandler.AsIndependentHandler()
		if !ok {
			return message.Endpoint{}, fmt.Errorf("service %q manager handler is not independent", service.Name)
		}
		return handler.Endpoint, nil
	}

	switch service.Type {
	case ProxyType:
		return manager.DefaultProxyManagerEndpoint(service.Name), nil
	case ExtensionType:
		return manager.DefaultExtensionManagerEndpoint(service.Name), nil
	case IndependentType:
		return DefaultServiceManagerEndpoint, nil
	default:
		return message.Endpoint{}, err
	}
}
