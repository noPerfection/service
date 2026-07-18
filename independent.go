package service

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	protocolClient "github.com/noPerfection/protocol/client"
	protocolHandler "github.com/noPerfection/protocol/handler"
	"github.com/noPerfection/protocol/handler/npac"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
	"github.com/noPerfection/service/manager"
	"github.com/noPerfection/service/mushroom"
	"github.com/noPerfection/service/package_url"
	"github.com/noPerfection/topology"
	"github.com/noPerfection/topology/config"
)

const DefaultName = "main"

// ManagerSecretKeyParameter is the service Parameters key for a hardcoded manager CURVE secret key.
// When present, the manager derives its public key from this value instead of generating a fresh pair.
const ManagerSecretKeyParameter = "manager-secret-key"

const DefaultConfigPath = "noPerfection.json"

var DefaultServiceManagerEndpoint = message.NewEndpoint(config.ServiceManagerCategory, 0)

// Independent keeps all necessary parameters of the independent service.
type Independent struct {
	*handlers.Setup
	*WithHardcodedTopology
	*TopologyConnection
	rawMushroomURL string
	mushroomURL    mushroom.TopologyURL
	blocker        *sync.WaitGroup
	manager        *manager.Manager // manage this service from other parts
	logger         *log.Logger
	aiClient       *AiClient
	ipcStarted     map[string]struct{}
}

// Follows pkg:golang/github.com/noPerfection/service?object=Service&root=no_perfection.go
func (independent *Independent) isService() {}

func (independent *Independent) AsIndependent() (*Independent, bool) {
	if independent == nil {
		return nil, false
	}
	return independent, true
}

func (independent *Independent) AsProxy() (*Proxy, bool) {
	return nil, false
}

func (independent *Independent) AsExtension() (*Extension, bool) {
	return nil, false
}

// New returns an independent service instance.
//
// Optional parameter:
//
//  1. mushroomURL — service identity in the configuration. A plain symbol is treated as the
//     service name at the root of the topology (e.g. "main" → services[name:main]). Full
//     mushroom paths are accepted but not validated yet.
//
// Use SetTopologyParams before Start to configure the topology JSON path.
//
//	// Root service "main".
//	app, err := New("main")
func New(params ...any) (*Independent, error) {
	mushroomURL := DefaultName

	if len(params) > 1 {
		return nil, fmt.Errorf("too many arguments, expected at most one service name")
	}

	if len(params) > 0 && params[0] != nil {
		mushroomUrlArg, ok := params[0].(string)
		if !ok {
			return nil, fmt.Errorf("name argument must be string")
		}
		if len(mushroomUrlArg) > 0 {
			mushroomURL = mushroomUrlArg
		}
	}

	independent := &Independent{
		Setup:                 handlers.NewSetup(),
		WithHardcodedTopology: NewHardcodedTopologies(mushroomURL),
		TopologyConnection:    newTopologyConnection(),
		rawMushroomURL:        mushroomURL,
		mushroomURL:           mushroom.TopologyURL{},
		logger:                nil,
	}

	return independent, nil
}

// EnableLogger toggles the optional service logger.
func (independent *Independent) EnableLogger(enable bool) error {
	if !enable {
		if err := independent.Setup.SetLogger(nil); err != nil {
			return fmt.Errorf("handlers.SetLogger: %w", err)
		}
		if independent.manager != nil {
			if err := independent.manager.SetLogger(nil); err != nil {
				return fmt.Errorf("manager.SetLogger: %w", err)
			}
		}
		independent.logger = nil
		return nil
	}

	logger, err := log.New(independent.rawMushroomURL, true)
	if err != nil {
		return fmt.Errorf("log.New(%s): %w", independent.rawMushroomURL, err)
	}
	if err := independent.Setup.SetLogger(logger); err != nil {
		return fmt.Errorf("handlers.SetLogger: %w", err)
	}

	if independent.manager != nil {
		if err := independent.manager.SetLogger(logger); err != nil {
			return fmt.Errorf("manager.SetLogger: %w", err)
		}
	}
	independent.logger = logger
	return nil
}

func (independent *Independent) dereference() string {
	if independent.mushroomURL.String() == "" {
		panic("Independent.mushroomURL is not set")
	}
	return independent.mushroomURL.AsDereference().String()
}

// addDefaultServiceToTopology adds the default service config
// if no config was given for this service.
func (independent *Independent) addDefaultServiceToTopology() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.dereference())
	if err == nil {
		return nil
	}

	serviceConfig = config.Service{
		Type:     config.IndependentType,
		Name:     independent.rawMushroomURL,
		Handlers: []config.Handler{},
	}

	moduleURL, err := package_url.FillDefaultModuleURL()
	if err != nil {
		return err
	}
	serviceConfig.ModuleUrl = moduleURL

	if err := tp.AddService(serviceConfig); err != nil {
		return fmt.Errorf("topology.AddService('%s'): %w", independent.dereference(), err)
	}

	return nil
}

// addDefaultHandlerToTopology adds the default handler when no handlers exist:
//
//   - Category: handlers.DefaultHandlerCategory
//   - Endpoint: handlers.DefaultHandlerEndpoint
//   - Type: ReplierType
func (independent *Independent) addDefaultHandlerToTopology() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.dereference())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.dereference(), err)
	}
	if len(serviceConfig.Handlers) > 0 {
		return nil
	}

	_, err = serviceConfig.HandlerByCategory(handlers.DefaultHandlerCategory)
	// No error indicates the default handler already exists
	if err == nil {
		return nil
	}

	defaultHandler := config.IndependentHandler{
		Category: handlers.DefaultHandlerCategory,
		Endpoint: handlers.DefaultHandlerEndpoint,
		Type:     config.ReplierType,
	}
	serviceConfig.Handlers = []config.Handler{defaultHandler}
	if err := tp.SetService(serviceConfig); err != nil {
		return fmt.Errorf("topology.SetService('%s'): %w", independent.dereference(), err)
	}

	return nil
}

// ensureServiceManager creates the service manager from topology configuration.
// When the service record has a manager handler, that endpoint is used;
// otherwise DefaultServiceManagerEndpoint is used.
func (independent *Independent) ensureServiceManager() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.dereference())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.dereference(), err)
	}

	managerEndpoint := DefaultServiceManagerEndpoint
	currentManager, err := serviceConfig.HandlerByCategory(config.ServiceManagerCategory)
	if err == nil {
		handler := currentManager.(config.IndependentHandler)
		managerEndpoint = handler.Endpoint
	}

	var secretKey string
	if serviceConfig.Parameters != nil {
		if v, ok := serviceConfig.Parameters[ManagerSecretKeyParameter]; ok {
			secretKey, _ = v.(string)
		}
	}

	m, err := manager.New(independent.mushroomURL, managerEndpoint, secretKey)
	if err != nil {
		return fmt.Errorf("manager.New: %w", err)
	}
	independent.manager = m
	if err := independent.manager.SetLogger(independent.logger); err != nil {
		return fmt.Errorf("manager.SetLogger: %w", err)
	}

	if err := independent.addAllowedManagerClients(serviceConfig.Parameters); err != nil {
		return fmt.Errorf("addAllowedManagerClients: %w", err)
	}

	return nil
}

func newHandler(handlerType config.HandlerType) (protocolHandler.Interface, error) {
	switch handlerType {
	case config.SyncReplierType:
		return protocolHandler.NewSyncReplier(), nil
	case config.ReplierType:
		return protocolHandler.NewReplier(), nil
	case config.PublisherType:
		return protocolHandler.NewPublisher(), nil
	case config.PairType:
		return protocolHandler.NewPair(), nil
	case config.WorkerType:
		return protocolHandler.NewWorker(), nil
	default:
		return nil, fmt.Errorf("unsupported handler type: %s", handlerType)
	}
}

// addTopologyHandlersToHandlers adds the handlers to the handlers list.
// Except for the Service Manager category, any handler defined in the topology is
// registered in the handlers package for launching them.
func (independent *Independent) addTopologyHandlersToHandlers() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.dereference())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.dereference(), err)
	}

	for _, configuredVariant := range serviceConfig.Handlers {
		configured, ok := configuredVariant.AsIndependentHandler()
		if !ok {
			continue
		}
		if configured.Category == config.ServiceManagerCategory {
			continue
		}

		handler, err := newHandler(configured.Type)
		if err != nil {
			return fmt.Errorf("newTopologyHandler('%s'): %w", configured.Category, err)
		}
		handler.SetEndpoint(configured.Endpoint)
		if err := independent.Setup.SetHandler(configured.Category, handler); err != nil {
			return fmt.Errorf("handlers.SetHandler('%s'): %w", configured.Category, err)
		}
	}

	return nil
}

// Start the service.
//
// Requires at least one handler.
func (independent *Independent) Start() error {
	var err error
	var inprocServices int
	var needToStart []config.Service
	var topologySnapshot string = ""
	var serviceLink string
	var tp topology.TopologyInterface

	if err = npac.New().Start(); err != nil {
		err = fmt.Errorf("npac.Start: %w", err)
		goto errOccurred
	}

	if err = independent.setupTopologyConnection(); err != nil {
		err = fmt.Errorf("setupTopologyConnection: %w", err)
		goto errOccurred
	}
	serviceLink, err = independent.topology().GetLink(independent.rawMushroomURL)
	if err != nil {
		err = fmt.Errorf("topology.GetLink('%s'): %w", independent.rawMushroomURL, err)
		goto errOccurred
	} else {
		independent.mushroomURL, err = mushroom.New(serviceLink)
		if err != nil {
			err = fmt.Errorf("mushroom.New('%s'): %w", serviceLink, err)
			goto errOccurred
		}
	}

	topologySnapshot, err = independent.topology().Snapshot()
	if err != nil {
		err = fmt.Errorf("topology.Snapshot: %w", err)
		goto errOccurred
	}
	if err = independent.WithHardcodedTopology.addHardcodedServicesToTopology(independent.topology()); err != nil {
		err = fmt.Errorf("addHardcodedServicesToTopology: %w", err)
		goto errOccurred
	}
	if err = independent.addDefaultServiceToTopology(); err != nil {
		err = fmt.Errorf("addDefaultServiceToTopology: %w", err)
		goto errOccurred
	}
	if err = independent.WithHardcodedTopology.addHardcodedHandlersToTopology(independent.topology()); err != nil {
		err = fmt.Errorf("addHardcodedHandlersToTopology: %w", err)
		goto errOccurred
	}
	if err = independent.addDefaultHandlerToTopology(); err != nil {
		err = fmt.Errorf("addDefaultHandlerToTopology: %w", err)
		goto errOccurred
	}

	if err = independent.addHardcodedHandlerDepsToTopology(independent.topology()); err != nil {
		err = fmt.Errorf("addHardcodedHandlerDepsToTopology: %w", err)
		goto errOccurred
	}
	if err = independent.WithHardcodedTopology.addHardcodedServiceParamsToTopology(independent.topology()); err != nil {
		err = fmt.Errorf("addHardcodedServiceParamsToTopology: %w", err)
		goto errOccurred
	}
	if err = independent.WithHardcodedTopology.addHardcodedEndpointsToTopology(independent.topology()); err != nil {
		err = fmt.Errorf("addHardcodedEndpointsToTopology: %w", err)
		goto errOccurred
	}

	// We call it after hardcoded stuff, just in case if user passed hardcoded manager data.
	if err = independent.ensureServiceManager(); err != nil {
		err = fmt.Errorf("ensureServiceManager: %w", err)
		goto errOccurred
	}

	if err = independent.WithHardcodedTopology.addHardcodedCommandDepsToTopology(independent.topology()); err != nil {
		err = fmt.Errorf("addHardcodedCommandDepsToTopology: %w", err)
		goto errOccurred
	}

	if err = independent.addTopologyHandlersToHandlers(); err != nil {
		err = fmt.Errorf("addTopologyHandlers: %w", err)
		goto errOccurred
	}

	// Managers must have the public keys
	if independent.manager.PublicKey() == "" {
		err = fmt.Errorf("manager.PublicKey() is empty")
		goto errOccurred
	}

	if err = independent.allowServiceManager(); err != nil {
		err = fmt.Errorf("allowServiceManager: %w", err)
		goto errOccurred
	}

	if independent.topologyHandler != nil {
		if err = independent.topologyHandler.Start(); err != nil {
			err = fmt.Errorf("topologyHandler.Start(): %w", err)
			goto errOccurred
		}
	}
	if err = independent.ensureTopologyClient(); err != nil {
		err = fmt.Errorf("ensureTopologyClient: %w", err)
		goto errOccurred
	}
	if err = independent.Setup.Start(independent.mushroomURL); err != nil {
		err = fmt.Errorf("handlers.Start: %w", err)
		goto errOccurred
	}

	independent.blocker = &sync.WaitGroup{}
	independent.blocker.Add(1)

	independent.manager.SetSharedBlocker(&independent.blocker)
	if err = independent.manager.Start(); err != nil {
		err = fmt.Errorf("service.manager.Start: %w", err)
		goto errOccurred
	}

	if err = independent.syncCommandOutbounds(); err != nil {
		err = fmt.Errorf("syncCommandOutbounds: %w", err)
		goto errOccurred
	}
	if err = independent.syncHandlerDepOutbounds(); err != nil {
		err = fmt.Errorf("syncHandlerDepOutbounds: %w", err)
		goto errOccurred
	}

	tp = independent.topology()
	if err = tp.ValidateProtocolOrder(independent.dereference()); err != nil {
		err = fmt.Errorf("topology.ValidateProtocolOrder: %w", err)
		goto errOccurred
	}
	if err = tp.ValidateInprocServiceManagers(); err != nil {
		err = fmt.Errorf("topology.ValidateInprocServiceManagers: %w", err)
		goto errOccurred
	}
	if inprocServices, err = tp.InprocessDepNumber(independent.dereference()); err != nil {
		err = fmt.Errorf("topology.InprocessDepNumber: %w", err)
		goto errOccurred
	}

	if inprocServices > 0 {
		needToStart, err = independent.startInproc()
		if err != nil {
			err = fmt.Errorf("setupInproc: %w", err)
			goto errOccurred
		}
	}
	if err = independent.cleanupInproc(needToStart); err != nil {
		err = fmt.Errorf("cleanupInproc: %w", err)
		goto errOccurred
	}
	if err = independent.startIpcServices(); err != nil {
		err = fmt.Errorf("startIpcServices: %w", err)
		goto errOccurred
	}
	if err = independent.registerOutbounds(); err != nil {
		err = fmt.Errorf("registerOutbounds: %w", err)
		goto errOccurred
	}
	// if err = independent.manager.AddTopologyManagers(); err != nil {
	// 	err = fmt.Errorf("addTopologyManagers: %w", err)
	// 	goto errOccurred
	// }
	// Wait for all IPC deps concurrently, reloading config on each probe so
	// that public keys written by newly started services are discovered.
	if err = independent.manager.Handshake(); err != nil {
		err = fmt.Errorf("manager.Handshake: %w", err)
		goto errOccurred
	}
	if err = independent.secureEdges(); err != nil {
		err = fmt.Errorf("secureEdges: %w", err)
		goto errOccurred
	}

errOccurred:
	if err != nil {
		if !IsNeedToRerunErr(err) && topologySnapshot != "" {
			if rollbackErr := independent.topology().Rollback(topologySnapshot); rollbackErr != nil {
				err = fmt.Errorf("%w: topology.Rollback: %v", err, rollbackErr)
			}
		}
		if topologyCloseErr := independent.closeTopologyClient(); topologyCloseErr != nil {
			err = fmt.Errorf("%w: closeTopologyClient: %w", err, topologyCloseErr)
		}
		if independent.manager != nil && independent.manager.Running() {
			closeErr := independent.manager.StopService(independent.dereference())
			if closeErr != nil {
				err = fmt.Errorf("%v: manager.StopService: %w", err, closeErr)
			}
		}
	}

	return err
}

func (independent *Independent) cleanupInproc(needToStart []config.Service) error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.dereference())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.dereference(), err)
	}
	if serviceConfig.ModuleUrl == "" {
		return fmt.Errorf("no mushroom url for service %q", independent.dereference())
	}

	registered, err := getInprocServices(serviceConfig.ModuleUrl)
	if err != nil {
		return fmt.Errorf("getInprocServices: %w", err)
	}

	wantNames := make(map[string]struct{}, len(needToStart))
	for _, svc := range needToStart {
		wantNames[svc.Name] = struct{}{}
	}

	needToRemove := make([]string, 0)
	for _, name := range registered {
		if name == "" {
			continue
		}
		svc, err := tp.Service(name)
		if err != nil {
			needToRemove = append(needToRemove, name)
			continue
		}
		if _, ok := wantNames[svc.Name]; !ok {
			needToRemove = append(needToRemove, svc.Name)
		}
	}

	if len(needToRemove) == 0 {
		return nil
	}

	inprocTopologyBackup, inprocTopologyFileExisted, err := readInprocTopologyFileContent(serviceConfig.ModuleUrl)
	if err != nil {
		return fmt.Errorf("readInprocTopologyFileContent: %w", err)
	}

	edited, err := RemoveInprocTopologyServices(serviceConfig.ModuleUrl, needToRemove)
	if err != nil {
		return fmt.Errorf("RemoveInprocTopologyServices: %w", err)
	}
	if !edited {
		return nil
	}

	remaining, err := getInprocServices(serviceConfig.ModuleUrl)
	if err != nil {
		return fmt.Errorf("getInprocServices: %w", err)
	}
	if len(remaining) == 0 {
		var failErr error

		inprocTopology, err := tp.Service(InprocTopologyServiceName)
		if err != nil {
			return fmt.Errorf("topology.Service(%q): %w", InprocTopologyServiceName, err)
		}
		topologyRunning, err := ProbeInprocServiceRunning(inprocTopology)
		if err != nil {
			return fmt.Errorf("probe inproc topology: %w", err)
		}
		if topologyRunning {
			if err := tp.StopService(InprocTopologyServiceName); err != nil {
				return fmt.Errorf("topology.StopService(%q): %w", InprocTopologyServiceName, err)
			}
		}

		if failErr = independent.removeInprocTopologyExtension(&serviceConfig); failErr != nil {
			goto rollbackInprocTeardown
		}

		if failErr = tp.RemoveService(InprocTopologyServiceName); failErr != nil {
			goto rollbackInprocTeardown
		}

		if failErr = DeleteInprocTopologyFile(serviceConfig.ModuleUrl); failErr != nil {
			goto rollbackInprocTeardown
		}

		if _, failErr = RemoveStartInprocTopologyCall(serviceConfig.ModuleUrl); failErr != nil {
			goto rollbackInprocTeardown
		}
		return NeedToRerun("removed %s and inproc-topology wiring; please rebuild and re-run", inprocTopologyFilename)

	rollbackInprocTeardown:
		if err := restoreInprocTopologyFileContent(serviceConfig.ModuleUrl, inprocTopologyBackup, inprocTopologyFileExisted); err != nil {
			return fmt.Errorf("%w: rollback restoreInprocTopologyFileContent: %w", failErr, err)
		}
		return failErr
	}
	return NeedToRerun("reconciled %s; please rebuild and re-run", inprocTopologyFilename)
}

func (independent *Independent) startInproc() ([]config.Service, error) {
	tp := independent.topology()
	// serviceConfig is used to ensure ai extension, or to find the path to the inproc_topology.go
	serviceConfig, err := tp.Service(independent.dereference())
	if err != nil {
		return nil, fmt.Errorf("topology.Service('%s'): %w", independent.dereference(), err)
	}
	if serviceConfig.ModuleUrl == "" {
		return nil, fmt.Errorf("no mushroom url for service %q", independent.dereference())
	}

	needToImport := make([]config.Service, 0)
	needToStart := make([]config.Service, 0)
	services, err := tp.Services()
	if err != nil {
		return nil, fmt.Errorf("topology.Services: %w", err)
	}
	for _, service := range services {
		if service.Name == serviceConfig.Name {
			continue
		}
		if !service.IsInproc() {
			continue
		}
		if service.Name == AiServiceName || service.Name == InprocTopologyServiceName {
			continue
		}
		pkgInfo, err := package_url.New(service.ModuleUrl)
		if err != nil {
			return nil, fmt.Errorf("package_url.New(%s): %w", service.ModuleUrl, err)
		}
		if pkgInfo.IsMain() {
			if err := pkgInfo.EnsureEditable(); err != nil {
				if errors.Is(err, package_url.ErrThirdPartyNotEditable) {
					return nil, fmt.Errorf("%w: fork %q and add a replace directive in go.mod to edit it locally", err, pkgInfo.ImportClause())
				}
				return nil, err
			}
			asLibInfo, exists, err := MainPackageToLibraryPackage(pkgInfo)
			if err != nil {
				return nil, err
			}
			if !exists {
				if err := independent.ensureAiExtension(serviceConfig); err != nil {
					return nil, err
				}
				if err := MainPackageToLibraryAI(independent.aiClient, pkgInfo, asLibInfo); err != nil {
					return nil, fmt.Errorf("ai main package to library: %w", err)
				}
			}

			service.ModuleUrl = asLibInfo.String()
			if err := tp.SetService(service); err != nil {
				return nil, fmt.Errorf("topology.SetService(%q): %w", service.Name, err)
			}
			pkgInfo = asLibInfo

			// Find the main module, and update the hardcode module url to the library package url
			if err := SetHardcodedModuleURL(serviceConfig.ModuleUrl, service.Name, asLibInfo); err != nil {
				return nil, fmt.Errorf("SetHardcodedModuleURL(%q): %w: update module-url for %q to %q yourself in the host main package", service.Name, err, service.Name, asLibInfo.String())
			}
		}

		if importErr := IsInprocIncludedInMain(serviceConfig.ModuleUrl, pkgInfo); importErr != nil {
			if errors.Is(importErr, ErrNotImported) {
				needToImport = append(needToImport, service)
				continue
			}
			return nil, fmt.Errorf("IsInprocIncludedInMain(%q): %w", service.Name, importErr)
		}
		needToStart = append(needToStart, service)
	}
	if len(needToStart) == 0 && len(needToImport) == 0 {
		return nil, nil
	}

	if _, err := tp.Service(InprocTopologyServiceName); err != nil {
		if err := tp.AddService(defaultInprocTopologyExtensionServiceConfig()); err != nil {
			return needToStart, fmt.Errorf("topology.AddService(%q): %w", InprocTopologyServiceName, err)
		}
	}
	if err := independent.addInprocTopologyExtension(&serviceConfig); err != nil {
		return needToStart, err
	}

	if len(needToImport) > 0 {
		if err := UpdateInprocTopology(serviceConfig.ModuleUrl, needToImport); err != nil {
			return needToStart, fmt.Errorf("UpdateInprocTopology: %w", err)
		}
	}

	inprocTopology, err := tp.Service(InprocTopologyServiceName)
	if err != nil {
		return needToStart, fmt.Errorf("topology.Service(%q): %w", InprocTopologyServiceName, err)
	}
	topologyRunning, err := ProbeInprocServiceRunning(inprocTopology)
	if err != nil {
		return needToStart, fmt.Errorf("probe inproc topology: %w", err)
	}

	mainEdited := false
	if !topologyRunning {
		contains, err := HostMainSourceContains(serviceConfig.ModuleUrl, startInprocTopologyCall)
		if err != nil {
			return needToStart, fmt.Errorf("HostMainSourceContains: %w", err)
		}
		if contains {
			return needToStart, fmt.Errorf("%w: did you change %s?", ErrInprocTopologyPresentNotRunning, inprocTopologyFilename)
		}
		mainEdited, err = EnsureStartInprocTopologyCall(serviceConfig.ModuleUrl, serviceConfig.Name)
		if err != nil {
			return needToStart, fmt.Errorf("EnsureStartInprocTopologyCall: %w", err)
		}
	}

	switch {
	case mainEdited && len(needToImport) > 0:
		return needToStart, NeedToRerun("imported inproc services, generated %s, and added startInprocTopology() in %s; please rebuild and re-run", inprocTopologyFilename, serviceConfig.ModuleUrl)
	case mainEdited && len(needToImport) == 0:
		return needToStart, NeedToRerun("all inproc services are valid; added startInprocTopology() in %s; please rebuild and re-run", serviceConfig.ModuleUrl)
	case !mainEdited && len(needToImport) > 0:
		return needToStart, NeedToRerun("imported inproc services into %s / %s; please re-run the code", serviceConfig.ModuleUrl, inprocTopologyFilename)
	default:
		for _, service := range needToStart {
			if _, err := independent.manager.StartService(service.Name); err != nil {
				return needToStart, fmt.Errorf("manager.StartService(%q): %w", service.Name, err)
			}
			running, err := ProbeInprocServiceRunning(service)
			if err != nil {
				return needToStart, fmt.Errorf("probe inproc service %q: %w", service.Name, err)
			}
			if !running {
				return needToStart, fmt.Errorf("inproc service %q is not running", service.Name)
			}
		}
		return needToStart, nil
	}
}

func (independent *Independent) removeInprocTopologyExtension(serviceConfig *config.Service) error {
	if serviceConfig == nil {
		return fmt.Errorf("service config is nil")
	}
	tp := independent.topology()
	updated := false
	for i, dep := range serviceConfig.HandlerDeps {
		if dep.Name != config.ServiceManagerCategory {
			continue
		}
		filtered := make([]string, 0, len(dep.Extensions))
		for _, extension := range dep.Extensions {
			extensionMushroomURL, err := mushroom.New(extension)
			if err != nil {
				return fmt.Errorf("mushroom.New(%q): %w", extension, err)
			}
			svc, err := tp.Service(extensionMushroomURL.AsDereference().String())
			if err != nil {
				filtered = append(filtered, extension)
				continue
			}
			if svc.Name == InprocTopologyServiceName {
				updated = true
				continue
			}
			filtered = append(filtered, extension)
		}
		if !updated {
			return nil
		}
		serviceConfig.HandlerDeps[i].Extensions = filtered
		if err := tp.SetService(*serviceConfig); err != nil {
			return fmt.Errorf("topology.SetService(%q): %w", serviceConfig.Name, err)
		}
		return nil
	}
	return nil
}

// ensureAiExtension ensures that the ai extension is running and connected.
// If so, it sets the independent.aiClient to connect to the ai extension.
func (independent *Independent) ensureAiExtension(serviceConfig config.Service) error {
	if independent.aiClient != nil {
		return nil
	}
	aiServiceConfig, hasAiDep := independent.getAiExtensionFromConfig(serviceConfig)
	if !hasAiDep {
		return fmt.Errorf("ai extension is not linked: call the SetHandlerDeps(service.Dependency{Name: service.ServiceManagerCategory, Extensions: []string{%q}})", AiServiceName)
	}

	running, err := ProbeInprocServiceRunning(aiServiceConfig)
	if err != nil {
		return fmt.Errorf("probe ai extension: %w", err)
	}
	if !running {
		return fmt.Errorf("ai extension is not running: add ai, _ := service.NewAiService() in your main(), then call ai.Start()")
	}

	client, err := NewAiClient(aiServiceConfig)
	if err != nil {
		return err
	}
	independent.aiClient = client
	return nil
}

// addInprocTopologyExtension adds the inproc-topology handler dep when missing and saves topology.
func (independent *Independent) addInprocTopologyExtension(serviceConfig *config.Service) error {
	if serviceConfig == nil {
		return fmt.Errorf("service config is nil")
	}
	tp := independent.topology()
	link := inprocTopologyExtensionServiceLink()
	for i, dep := range serviceConfig.HandlerDeps {
		if dep.Name != config.ServiceManagerCategory {
			continue
		}
		for _, extension := range dep.Extensions {
			extensionMushroomURL, err := mushroom.New(extension)
			if err != nil {
				return fmt.Errorf("mushroom.New(%q): %w", extension, err)
			}
			svc, err := tp.Service(extensionMushroomURL.AsDereference().String())
			if err != nil {
				return fmt.Errorf("topology.Service(%q): %w", extension, err)
			}
			if svc.Name == InprocTopologyServiceName {
				return nil
			}
		}
		serviceConfig.HandlerDeps[i].Extensions = append(dep.Extensions, link)
		if err := tp.SetService(*serviceConfig); err != nil {
			return fmt.Errorf("topology.SetService(%q): %w", serviceConfig.Name, err)
		}
		return nil
	}

	serviceConfig.HandlerDeps = append(serviceConfig.HandlerDeps, config.DepService{
		Name:       config.ServiceManagerCategory,
		Extensions: []string{link},
	})
	if err := tp.SetService(*serviceConfig); err != nil {
		return fmt.Errorf("topology.SetService(%q): %w", serviceConfig.Name, err)
	}
	return nil
}

func (independent *Independent) getAiExtensionFromConfig(serviceConfig config.Service) (config.Service, bool) {
	for _, dep := range serviceConfig.HandlerDeps {
		if dep.Name != config.ServiceManagerCategory {
			continue
		}
		for _, link := range dep.Extensions {
			service, err := independent.topology().Service(link)
			if err != nil {
				continue
			}
			if service.Name == AiServiceName {
				return service, true
			}
		}
		return config.Service{}, false
	}
	return config.Service{}, false
}

// allowServiceManager publishes this service's manager public key in its own
// Parameters["public-key"] if it doesn't exist already.
//
// Then it publishes a dereference to that field
// in the "allowed" parameters of every dependency service (handler-deps and
// command-deps). Each dep service receives an entry:
//
// so that the dep service's manager handler can authenticate connections from
// this manager by resolving the reference at allow-time.
func (independent *Independent) allowServiceManager() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.dereference())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.dereference(), err)
	}
	managerLink := independent.mushroomURL.New(config.ServiceManagerCategory)

	publicKey := independent.manager.PublicKey()

	if serviceConfig.Parameters == nil {
		serviceConfig.Parameters = datatype.New()
	}
	if existing, _ := serviceConfig.Parameters[manager.ManagerPublicKeyParam].(string); existing != publicKey {
		serviceConfig.Parameters[manager.ManagerPublicKeyParam] = publicKey
		if err := tp.SetService(serviceConfig); err != nil {
			return fmt.Errorf("topology.SetService('%s') store public key: %w", serviceConfig.Name, err)
		}
	}

	depServiceURLs := make(map[string]struct{})

	for _, dep := range serviceConfig.HandlerDeps {
		for _, u := range dep.Proxies {
			link, err := tp.GetLink(u)
			if err != nil {
				return fmt.Errorf("topology.GetLink('%s'): %w", u, err)
			}
			depServiceURLs[link] = struct{}{}
		}
		for _, u := range dep.Extensions {
			link, err := tp.GetLink(u)
			if err != nil {
				return fmt.Errorf("topology.GetLink('%s'): %w", u, err)
			}
			depServiceURLs[link] = struct{}{}
		}
	}
	for _, variant := range serviceConfig.Handlers {
		handler, ok := variant.AsIndependentHandler()
		if !ok {
			continue
		}
		for _, dep := range handler.CommandDeps {
			for _, u := range dep.Proxies {
				link, err := tp.GetLink(u)
				if err != nil {
					return fmt.Errorf("topology.GetLink('%s'): %w", u, err)
				}
				depServiceURLs[link] = struct{}{}
			}
			for _, u := range dep.Extensions {
				link, err := tp.GetLink(u)
				if err != nil {
					return fmt.Errorf("topology.GetLink('%s'): %w", u, err)
				}
				depServiceURLs[link] = struct{}{}
			}
		}
	}

	for svcURL := range depServiceURLs {
		mushroomURL, err := mushroom.New(svcURL)
		if err != nil {
			return fmt.Errorf("mushroom.New(%q): %w", svcURL, err)
		}
		depService, err := tp.Service(mushroomURL.AsDereference().String())
		if err != nil {
			return fmt.Errorf("topology.Service('%s'): %w", svcURL, err)
		}
		if !mushroom.IsAllowedPublicKeyMatch(&depService, managerLink, independent.mushroomURL.ResourcePublicKey()) {
			continue
		}
		mushroom.AddAllowedPublicKey(&depService, managerLink, independent.mushroomURL.ResourcePublicKey())
		if err := tp.SetService(depService); err != nil {
			return fmt.Errorf("topology.SetService('%s'): %w", depService.Name, err)
		}
	}

	return nil
}

// addAllowedManagerClients reads the "allowed" parameters of this service and calls
// manager.Allow for every public key listed under the ServiceManagerCategory.
// The topology resolves dereference URLs (via Fruit) before returning the
// service config, so values are always plain key strings by the time they arrive
// here. Missing or empty allowed entries are logged as warnings.
func (independent *Independent) addAllowedManagerClients(parameters datatype.KeyValue) error {
	if parameters == nil {
		if independent.logger != nil {
			independent.logger.Warn("no allowed keys: parameters not set, no one can access this service", "service", independent.rawMushroomURL)
		}
		return nil
	}

	allowed, ok := parameters["allowed"]
	if !ok {
		if independent.logger != nil {
			independent.logger.Warn("no allowed keys: 'allowed' parameter missing, no one can access this service", "service", independent.rawMushroomURL)
		}
		return nil
	}

	categoryMap, ok := allowed.(map[string]interface{})
	if !ok {
		if independent.logger != nil {
			independent.logger.Warn("no allowed keys: 'allowed' parameter has unexpected type", "service", independent.rawMushroomURL)
		}
		return nil
	}

	managerEntry, ok := categoryMap[config.ServiceManagerCategory]
	if !ok {
		if independent.logger != nil {
			independent.logger.Warn("no allowed keys: service manager category not found in allowed", "service", independent.rawMushroomURL, "category", config.ServiceManagerCategory)
		}
		return nil
	}

	entryMap, ok := managerEntry.(map[string]interface{})
	if !ok {
		if independent.logger != nil {
			independent.logger.Warn("no allowed keys: manager allowed entry has unexpected type", "service", independent.rawMushroomURL)
		}
		return nil
	}

	for link, pubKeyVal := range entryMap {
		pubKey, ok := pubKeyVal.(string)
		if !ok || pubKey == "" {
			continue
		}
		independent.manager.Allow(pubKey)
		fmt.Printf("The %s allowed to access %s\n", link, independent.rawMushroomURL)
	}
	if len(entryMap) > 0 {
		independent.manager.RequireWhitelist()
	}

	return nil
}

func (independent *Independent) syncHandlerDepOutbounds() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.dereference())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.dereference(), err)
	}
	if len(serviceConfig.HandlerDeps) == 0 {
		return nil
	}

	for depIndex := range serviceConfig.HandlerDeps {
		dep := &serviceConfig.HandlerDeps[depIndex]
		if len(dep.Proxies) == 0 {
			continue
		}

		handlerVariant, err := serviceConfig.HandlerByCategory(dep.Name)
		if err != nil {
			return fmt.Errorf("handler dep %q: %w", dep.Name, err)
		}
		handler, ok := handlerVariant.AsIndependentHandler()
		if !ok {
			return fmt.Errorf("handler dep %q is not an independent handler", dep.Name)
		}
		routes, err := independent.Setup.RouteCommands(dep.Name)
		if err != nil {
			return fmt.Errorf("handler dep %q route commands: %w", dep.Name, err)
		}
		if len(routes) == 0 {
			continue
		}

		for proxyIndex := range dep.Proxies {
			proxyURL := dep.Proxies[proxyIndex]
			outbound, commandOutbounds, err := independent.handlerDepProxyOutboundTargets(handler, dep.Proxies, proxyIndex, routes)
			if err != nil {
				return fmt.Errorf("handler %q proxy %q outbound: %w", dep.Name, proxyURL, err)
			}
			if err := independent.syncHandlerDepProxyOutbounds(routes, proxyURL, outbound, commandOutbounds); err != nil {
				return fmt.Errorf("handler %q proxy %q: %w", dep.Name, proxyURL, err)
			}
		}
	}

	return nil
}

// startIpcServices starts IPC services this service depends on.
func (independent *Independent) startIpcServices() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.dereference())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.dereference(), err)
	}

	startedRefs := make(map[string]struct{})
	return independent.startIpcDepsFor(serviceConfig, startedRefs)
}

func (independent *Independent) startIpcDepsFor(serviceConfig config.Service, startedRefs map[string]struct{}) error {
	tp := independent.topology()
	for _, dep := range serviceConfig.HandlerDeps {
		for _, proxy := range dep.Proxies {
			link, err := tp.GetLink(proxy)
			if err != nil {
				return fmt.Errorf("topology.GetLink('%s'): %w", proxy, err)
			}
			proxyMushroomURL, err := mushroom.New(link)
			if err != nil {
				return fmt.Errorf("mushroom.New(%q): %w", proxy, err)
			}
			if err := independent.startIpcService(proxyMushroomURL, startedRefs); err != nil {
				return fmt.Errorf("HandlerDeps[name: %q].Proxies[%q]: %w", dep.Name, proxy, err)
			}
		}
		for _, extension := range dep.Extensions {
			link, err := tp.GetLink(extension)
			if err != nil {
				return fmt.Errorf("topology.GetLink('%s'): %w", extension, err)
			}
			extensionMushroomURL, err := mushroom.New(link)
			if err != nil {
				return fmt.Errorf("mushroom.New(%q): %w", extension, err)
			}
			if err := independent.startIpcService(extensionMushroomURL, startedRefs); err != nil {
				return fmt.Errorf("HandlerDeps[name: %q].Extensions[%q]: %w", dep.Name, extension, err)
			}
		}
	}

	for _, variant := range serviceConfig.Handlers {
		handler, ok := variant.AsIndependentHandler()
		if !ok {
			continue
		}
		for _, dep := range handler.CommandDeps {
			for _, proxy := range dep.Proxies {
				link, err := tp.GetLink(proxy)
				if err != nil {
					return fmt.Errorf("topology.GetLink('%s'): %w", proxy, err)
				}
				proxyMushroomURL, err := mushroom.New(link)
				if err != nil {
					return fmt.Errorf("mushroom.New(%q): %w", proxy, err)
				}
				if err := independent.startIpcService(proxyMushroomURL, startedRefs); err != nil {
					return fmt.Errorf("Handlers[category: %q].CommandDeps[name: %q].Proxies[%q]: %w", handler.Category, dep.Name, proxy, err)
				}
			}
			for _, extension := range dep.Extensions {
				link, err := tp.GetLink(extension)
				if err != nil {
					return fmt.Errorf("topology.GetLink('%s'): %w", extension, err)
				}
				extensionMushroomURL, err := mushroom.New(link)
				if err != nil {
					return fmt.Errorf("mushroom.New(%q): %w", extension, err)
				}
				if err := independent.startIpcService(extensionMushroomURL, startedRefs); err != nil {
					return fmt.Errorf("Handlers[category: %q].CommandDeps[name: %q].Extensions[%q]: %w", handler.Category, dep.Name, extension, err)
				}
			}
		}
	}

	return nil
}

func (independent *Independent) startIpcService(mushroomURL mushroom.TopologyURL, startedRefs map[string]struct{}) error {
	depService, err := independent.topology().Service(mushroomURL.AsDereference().String())
	if err != nil {
		return err
	}
	if _, done := startedRefs[depService.Name]; done {
		return nil
	}
	startedRefs[depService.Name] = struct{}{}

	if err := independent.startIpcDepsFor(depService, startedRefs); err != nil {
		return fmt.Errorf("service %q ipc deps: %w", depService.Name, err)
	}
	if !depService.IsIpc() {
		return nil
	}
	if len(depService.StartCommand) == 0 {
		return fmt.Errorf("service '%s' has no start command given", depService.Name)
	}

	derefURL := mushroomURL.AsDereference().String()
	running, err := independent.manager.IsServiceRunning(derefURL)
	if err != nil {
		if errors.Is(err, message.ErrAccessDenied) {
			return nil
		}
		return fmt.Errorf("manager.IsServiceRunning('%s'): %w", depService.Name, err)
	}
	if running {
		return nil
	}
	if _, err := independent.manager.StartService(depService.Name); err != nil {
		return fmt.Errorf("manager.StartService('%s'): %w", depService.Name, err)
	}
	independent.ipcStarted = markIpcStarted(independent.ipcStarted, depService.Name)
	running, err = independent.manager.IsServiceRunning(derefURL, 10)
	if err != nil {
		if errors.Is(err, message.ErrAccessDenied) {
			return nil
		}
		return fmt.Errorf("manager.IsServiceRunning('%s'): %w", depService.Name, err)
	}
	if !running {
		return fmt.Errorf("service %q did not become running after start", depService.Name)
	}

	return nil
}

func normalizeProxyHandlerOutbounds(handler config.Handler) config.Handler {
	proxyHandler, ok := handler.AsProxyHandler()
	if !ok || proxyHandler.Outbounds != nil {
		return handler
	}
	proxyHandler.Outbounds = []string{}
	return proxyHandler
}

// registerOutbounds registers every handler-dep, command-dep, and service-manager facade with npac
// so nested handler calls can resolve outbound CURVE keys and control endpoints.
//
// Doesn't registers manager related outbounds
func (independent *Independent) registerOutbounds() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.dereference())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.dereference(), err)
	}

	autocontext := protocolClient.NewAutocontext()
	if autocontext == nil {
		return fmt.Errorf("failed to create npac autocontext")
	}
	defer func() { _ = autocontext.Close() }()

	registered := make(map[string]struct{})
	registerFacade := func(facadeURL string) error {
		if facadeURL == "" {
			return nil
		}
		if _, done := registered[facadeURL]; done {
			return nil
		}
		endpoint, publicKey, err := independent.getEndpointAndPublicKey(facadeURL)
		if err != nil {
			return err
		}
		if err := autocontext.RegisterOutbound(endpoint, facadeURL, publicKey); err != nil {
			return fmt.Errorf("npac.RegisterOutbound(%q): %w", facadeURL, err)
		}
		registered[facadeURL] = struct{}{}
		return nil
	}

	registerDepManager := func(facadeURL string) error {
		managerMushroomURL, err := mushroom.New(facadeURL, config.ServiceManagerCategory)
		if err != nil {
			return fmt.Errorf("mushroom.New(%q): %w", facadeURL, err)
		}
		return registerFacade(managerMushroomURL.String())
	}

	for _, dep := range serviceConfig.HandlerDeps {
		for _, proxy := range dep.Proxies {
			link, err := tp.GetLink(proxy)
			if err != nil {
				return fmt.Errorf("topology.GetLink('%s'): %w", proxy, err)
			}
			proxyMushroomURL, err := mushroom.New(link)
			if err != nil {
				return fmt.Errorf("mushroom.New(%q): %w", link, err)
			}
			facadeURL, err := tp.GetFacade(proxyMushroomURL.AsDereference().String())
			if err != nil {
				return fmt.Errorf("HandlerDeps[name: %q].Proxies[%q]: %w", dep.Name, proxy, err)
			}
			if err := registerFacade(facadeURL); err != nil {
				return fmt.Errorf("HandlerDeps[name: %q].Proxies[%q]: %w", dep.Name, proxy, err)
			}
			if err := registerDepManager(facadeURL); err != nil {
				return fmt.Errorf("HandlerDeps[name: %q].Proxies[%q] manager: %w", dep.Name, proxy, err)
			}
		}
		for _, extension := range dep.Extensions {
			link, err := tp.GetLink(extension)
			if err != nil {
				return fmt.Errorf("topology.GetLink('%s'): %w", extension, err)
			}
			extensionMushroomURL, err := mushroom.New(link)
			if err != nil {
				return fmt.Errorf("mushroom.New(%q): %w", extension, err)
			}
			facadeURL, err := tp.GetFacade(extensionMushroomURL.AsDereference().String())
			if err != nil {
				return fmt.Errorf("HandlerDeps[name: %q].Extensions[%q]: %w", dep.Name, extension, err)
			}
			if err := registerFacade(facadeURL); err != nil {
				return fmt.Errorf("HandlerDeps[name: %q].Extensions[%q]: %w", dep.Name, extension, err)
			}
			if err := registerDepManager(facadeURL); err != nil {
				return fmt.Errorf("HandlerDeps[name: %q].Extensions[%q] manager: %w", dep.Name, extension, err)
			}
		}
	}

	for _, variant := range serviceConfig.Handlers {
		handler, ok := variant.AsIndependentHandler()
		if !ok || handler.Category == config.ServiceManagerCategory {
			continue
		}
		for _, dep := range handler.CommandDeps {
			for _, proxy := range dep.Proxies {
				link, err := tp.GetLink(proxy)
				if err != nil {
					return fmt.Errorf("topology.GetLink('%s'): %w", proxy, err)
				}
				proxyMushroomURL, err := mushroom.New(link)
				if err != nil {
					return fmt.Errorf("mushroom.New(%q): %w", link, err)
				}
				facadeURL, err := tp.GetFacade(proxyMushroomURL.AsDereference().String(), dep.Name)
				if err != nil {
					return fmt.Errorf("GetFacade(%q): %w", proxyMushroomURL.AsDereference().String(), err)
				}
				if err := registerFacade(facadeURL); err != nil {
					return fmt.Errorf("Handlers[category: %q].CommandDeps[name: %q].Proxies[%q]: %w", handler.Category, dep.Name, proxy, err)
				}
				if err := registerDepManager(facadeURL); err != nil {
					return fmt.Errorf("Handlers[category: %q].CommandDeps[name: %q].Proxies[%q] manager: %w", handler.Category, dep.Name, proxy, err)
				}
			}
			for _, extension := range dep.Extensions {
				link, err := tp.GetLink(extension)
				if err != nil {
					return fmt.Errorf("topology.GetLink('%s'): %w", extension, err)
				}
				extensionMushroomURL, err := mushroom.New(link)
				if err != nil {
					return fmt.Errorf("mushroom.New(%q): %w", link, err)
				}
				facadeURL, err := tp.GetFacade(extensionMushroomURL.AsDereference().String(), dep.Name)
				if err != nil {
					return fmt.Errorf("Handlers[category: %q].CommandDeps[name: %q].Extensions[%q]: %w", handler.Category, dep.Name, extension, err)
				}
				if err := registerFacade(facadeURL); err != nil {
					return fmt.Errorf("Handlers[category: %q].CommandDeps[name: %q].Extensions[%q]: %w", handler.Category, dep.Name, extension, err)
				}
				if err := registerDepManager(facadeURL); err != nil {
					return fmt.Errorf("Handlers[category: %q].CommandDeps[name: %q].Extensions[%q] manager: %w", handler.Category, dep.Name, extension, err)
				}
			}
		}
	}

	return nil
}

func (independent *Independent) getEndpointAndPublicKey(facadeURL string) (message.Endpoint, string, error) {
	handlerConfig, err := independent.resolveTopologyHandler(facadeURL)
	if err != nil {
		return message.Endpoint{}, "", fmt.Errorf("resolveTopologyHandler(%q): %w", facadeURL, err)
	}
	ind, ok := handlerConfig.AsIndependentHandler()
	if !ok {
		return message.Endpoint{}, "", fmt.Errorf("facade %q is not an independent handler", facadeURL)
	}

	publicKey := ""
	mushroomURL, err := mushroom.Parse(facadeURL)
	if err != nil {
		return message.Endpoint{}, "", fmt.Errorf("mushroom.Parse(%q): %w", facadeURL, err)
	}
	depService, err := independent.topology().Service(mushroomURL.As(mushroom.SERVICE).AsDereference().String())
	if err == nil && depService.Parameters != nil {
		if pk, ok := depService.Parameters[manager.ManagerPublicKeyParam].(string); ok {
			publicKey = pk
		}
	}

	return ind.Endpoint, publicKey, nil
}

// For every proxy in a command’s chain, figure out who it forwards to,
// write that into the proxy’s config, save it, and tell the running proxy to reload.
func (independent *Independent) syncCommandOutbounds() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.dereference())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.dereference(), err)
	}

	for handlerIndex := range serviceConfig.Handlers {
		handler, _ := serviceConfig.Handlers[handlerIndex].AsIndependentHandler()
		if handler.Category == config.ServiceManagerCategory || len(handler.CommandDeps) == 0 {
			continue
		}

		for depIndex := range handler.CommandDeps {
			dep := &handler.CommandDeps[depIndex]
			for proxyIndex := range dep.Proxies {
				proxyURL := dep.Proxies[proxyIndex]
				var outboundURL string
				if proxyIndex+1 < len(dep.Proxies) {
					proxyMushroomURL, err := mushroom.New(dep.Proxies[proxyIndex+1])
					if err != nil {
						return fmt.Errorf("mushroom.New(%q): %w", dep.Proxies[proxyIndex+1], err)
					}
					outboundURL, err = tp.GetFacade(proxyMushroomURL.AsDereference().String(), dep.Name)
					if err != nil {
						return fmt.Errorf("GetFacade(%q): %w", proxyMushroomURL.AsDereference().String(), err)
					}
				} else {
					outboundURL = independent.mushroomURL.New(handler.Category).String()
				}
				fmt.Println("syncCommandProxyOutbound: cmd=", dep.Name, "proxy-url=", proxyURL, "outbound-url=", outboundURL)
				if err := independent.syncCommandProxyOutbound(dep.Name, proxyURL, outboundURL); err != nil {
					return fmt.Errorf("handler %q command %q proxy %q: %w", handler.Category, dep.Name, proxyURL, err)
				}
			}
		}
	}

	return nil
}

// For a handler depenency get the outbound:
// 1) If there are another handler dependency, get that service facade
// 2) If there are routes that matches the command deps, then get that outbound as secondary outbounds
// 3) If no deps then get the service itself
func (independent *Independent) handlerDepProxyOutboundTargets(handlerConfig config.Handler, proxies []string, proxyIndex int, routes []string) (string, map[string]string, error) {
	tp := independent.topology()
	if proxyIndex+1 < len(proxies) {
		proxyMushroomURL, err := mushroom.New(proxies[proxyIndex+1])
		if err != nil {
			return "", nil, fmt.Errorf("mushroom.New(%q): %w", proxies[proxyIndex+1], err)
		}
		outboundURL, err := tp.GetFacade(proxyMushroomURL.AsDereference().String())
		return outboundURL, nil, err
	}

	commandOutbounds := make(map[string]string)
	for _, route := range routes {
		commandDep, ok := commandDepByName(handlerConfig, route)
		if !ok || len(commandDep.Proxies) == 0 {
			continue
		}
		proxyMushroomURL, err := mushroom.New(commandDep.Proxies[0])
		if err != nil {
			return "", nil, fmt.Errorf("mushroom.New(%q): %w", commandDep.Proxies[0], err)
		}
		outboundURL, err := tp.GetFacade(proxyMushroomURL.AsDereference().String())
		if err != nil {
			return "", nil, fmt.Errorf("command %q first proxy: %w", route, err)
		}
		commandOutbounds[route] = outboundURL
	}

	handler, ok := handlerConfig.AsIndependentHandler()
	if !ok {
		return "", nil, fmt.Errorf("handler is not an independent handler")
	}
	outboundURL := independent.mushroomURL.New(handler.Category)
	return outboundURL.String(), commandOutbounds, nil
}

func (independent *Independent) syncHandlerDepProxyOutbounds(routes []string, proxyHandlerUrl string, outboundURL string, commandOutbounds map[string]string) error {
	handler, err := independent.resolveTopologyHandler(proxyHandlerUrl)
	if err != nil {
		return err
	}
	proxyHandler, ok := handler.AsProxyHandler()
	if !ok {
		return fmt.Errorf("dep %q is not a proxy handler", proxyHandlerUrl)
	}
	proxyHandler, ok = normalizeProxyHandlerOutbounds(proxyHandler).AsProxyHandler()
	if !ok {
		return fmt.Errorf("dep %q is not a proxy handler", proxyHandlerUrl)
	}
	updated := false
	if !stringSlicesEqual(proxyHandler.Routes, routes) {
		proxyHandler.Routes = append([]string(nil), routes...)
		updated = true
	}
	outbounds := append([]string(nil), proxyHandler.Outbounds...)
	if outboundURL != "" {
		outbounds = appendUnique(outbounds, outboundURL)
	}
	for command, commandOutboundURL := range commandOutbounds {
		outbounds = appendUnique(outbounds, commandOutboundURL)
		var updatedForward bool
		proxyHandler, updatedForward = ensureProxyHandlerForward(proxyHandler, command, commandOutboundURL)
		updated = updated || updatedForward
	}
	for _, forwardURL := range proxyHandler.Forward {
		outbounds = appendUnique(outbounds, forwardURL)
	}
	if !stringSlicesEqual(proxyHandler.Outbounds, outbounds) {
		proxyHandler.Outbounds = outbounds
		updated = true
	}
	if updated {
		if err := independent.setTopologyHandler(proxyHandler, proxyHandlerUrl); err != nil {
			return err
		}
	}
	return nil
}

func commandDepByName(handlerConfig config.Handler, command string) (config.DepService, bool) {
	handler, ok := handlerConfig.AsIndependentHandler()
	if !ok {
		return config.DepService{}, false
	}
	for _, dep := range handler.CommandDeps {
		if dep.Name == command {
			return dep, true
		}
	}
	return config.DepService{}, false
}

func (independent *Independent) syncCommandProxyOutbound(command string, proxyHandlerUrl string, outboundURL string) error {
	handler, err := independent.resolveTopologyHandler(proxyHandlerUrl)
	if err != nil {
		return err
	}
	proxyHandler, ok := handler.AsProxyHandler()
	if !ok {
		return fmt.Errorf("dep %q is not a proxy handler", proxyHandlerUrl)
	}
	proxyHandler, ok = normalizeProxyHandlerOutbounds(proxyHandler).AsProxyHandler()
	if !ok {
		return fmt.Errorf("dep %q is not a proxy handler", proxyHandlerUrl)
	}
	updated := false
	if !containsString(proxyHandler.Routes, command) {
		proxyHandler.Routes = append(proxyHandler.Routes, command)
		updated = true
	}
	updatedOutbound := proxyHandler.SetOutbound(outboundURL)
	updated = updated || updatedOutbound
	var updatedForward bool
	proxyHandler, updatedForward = ensureProxyHandlerForward(proxyHandler, command, outboundURL)
	updated = updated || updatedForward

	if updated {
		if err := independent.setTopologyHandler(proxyHandler, proxyHandlerUrl); err != nil {
			return err
		}
	}
	return nil
}

func ensureProxyHandlerForward(proxyConfig config.ProxyHandler, command string, outboundURL string) (config.ProxyHandler, bool) {
	if proxyConfig.Forward == nil {
		proxyConfig.Forward = make(map[string]string)
	}
	if proxyConfig.Forward[command] == outboundURL {
		return proxyConfig, false
	}
	proxyConfig.Forward[command] = outboundURL
	proxyConfig.SetOutbound(outboundURL)
	return proxyConfig, true
}

func stringSlicesEqual(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func newProxyManagerClient(proxyService config.Service) (*protocolClient.SyncReplierClient, error) {
	endpoint := manager.DefaultProxyManagerEndpoint(proxyService.Name)
	if managerHandler, err := proxyService.HandlerByCategory(config.ServiceManagerCategory); err == nil {
		handler, ok := managerHandler.AsIndependentHandler()
		if ok {
			endpoint = handler.Endpoint
		}
	}
	client, err := protocolClient.NewSyncReplier(endpoint.Id, endpoint.Port)
	if err != nil {
		return nil, err
	}
	client.Timeout(time.Second)
	client.Attempt(1)
	return client, nil
}

func proxyHandlerExists(client *protocolClient.SyncReplierClient, serviceName string, category string) (bool, error) {
	reply, err := proxyManagerRequest(client, handlers.IsProxyHandlerExistCommand, datatype.New().Set("service", serviceName).Set("category", category))
	if err != nil {
		return false, err
	}
	return reply.ReplyParameters().BoolValue("exists")
}

func proxyHandlerRunning(client *protocolClient.SyncReplierClient, serviceName string, category string) (bool, error) {
	reply, err := proxyManagerRequest(client, handlers.IsProxyHandlerRunningCommand, datatype.New().Set("service", serviceName).Set("category", category))
	if err != nil {
		return false, err
	}
	return reply.ReplyParameters().BoolValue("running")
}

func setProxyHandler(client *protocolClient.SyncReplierClient, serviceName string, proxyConfig config.ProxyHandler) error {
	configParams, err := datatype.NewFromInterface(proxyConfig)
	if err != nil {
		return fmt.Errorf("datatype.NewFromInterface: %w", err)
	}
	_, err = proxyManagerRequest(client, handlers.SetProxyHandlerCommand, datatype.New().Set("service", serviceName).Set("config", configParams))
	return err
}

func startProxyHandler(client *protocolClient.SyncReplierClient, serviceName string, category string) error {
	_, err := proxyManagerRequest(client, handlers.StartProxyHandlerCommand, datatype.New().Set("service", serviceName).Set("category", category))
	return err
}

func stopProxyHandler(client *protocolClient.SyncReplierClient, serviceName string, category string) error {
	_, err := proxyManagerRequest(client, handlers.StopProxyHandlerCommand, datatype.New().Set("service", serviceName).Set("category", category))
	return err
}

func proxyManagerRequest(client *protocolClient.SyncReplierClient, command string, params datatype.KeyValue) (message.ReplyInterface, error) {
	reply, err := client.Request(&message.Request{
		Command:    command,
		Parameters: params,
	})
	if err != nil {
		return nil, fmt.Errorf("proxy manager request %q: %w", command, err)
	}
	if !reply.IsOK() {
		return nil, fmt.Errorf("proxy manager request %q: %s", command, reply.ErrorMessage())
	}
	return reply, nil
}

func appendUnique(values []string, value string) []string {
	if containsString(values, value) {
		return values
	}
	return append(values, value)
}

func containsString(values []string, value string) bool {
	for _, existing := range values {
		if existing == value {
			return true
		}
	}
	return false
}

func (independent *Independent) stopIpcServices() error {
	tp := independent.topology()
	serviceConfig, err := tp.Service(independent.dereference())
	if err != nil {
		return fmt.Errorf("topology.Service('%s'): %w", independent.dereference(), err)
	}

	lifecycle := &ipcLifecycle{
		topology: tp,
		manager:  independent.manager,
		started:  independent.ipcStarted,
	}
	return lifecycle.stopOwnedIpcServices(serviceConfig)
}

func (independent *Independent) Stop() error {
	if independent.manager != nil && !independent.manager.Running() {
		return nil
	}

	var stopErr error
	if err := independent.stopIpcServices(); err != nil {
		stopErr = joinStopErrors(stopErr, err)
	}
	if independent.topologyHandler != nil {
		if err := independent.topologyHandler.StopAllSpawnedProcesses(); err != nil {
			stopErr = joinStopErrors(stopErr, err)
		}
	}
	if independent.topologyClient != nil {
		_ = independent.topologyClient.Close()
		independent.topologyClient = nil
	}
	if err := independent.manager.StopService(independent.dereference()); err != nil {
		stopErr = joinStopErrors(stopErr, err)
	}
	return stopErr
}

func (independent *Independent) Wait() {
	waitForShutdown(independent.blocker, independent.Stop)
}
