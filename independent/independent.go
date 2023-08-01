/*Package independent is used to scaffold the independent service
 */
package independent

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/communication/command"
	"github.com/ahmetson/service-lib/communication/message"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/configuration/argument"
	"github.com/ahmetson/service-lib/configuration/path"
	"github.com/ahmetson/service-lib/context/dev"
	"github.com/ahmetson/service-lib/controller"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/service-lib/remote"
	"os"
	"strings"
	"sync"
)

// Service is the collection of the various Controllers
type Service struct {
	Config          *configuration.Config
	Controllers     key_value.KeyValue
	Pipelines       key_value.KeyValue
	RequiredProxies key_value.KeyValue // url => context type
	Logger          *log.Logger
	Context         *dev.Context
	manager         controller.Interface // manage this service from other parts. it should be called before context run
}

// New service based on the configurations
func New(config *configuration.Config, logger *log.Logger) (*Service, error) {
	independent := Service{
		Config:          config,
		Logger:          logger,
		Controllers:     key_value.Empty(),
		RequiredProxies: key_value.Empty(),
		Pipelines:       key_value.Empty(),
	}

	return &independent, nil
}

// AddController by their instance name
func (independent *Service) AddController(name string, controller controller.Interface) {
	independent.Controllers.Set(name, controller)
}

func (independent *Service) RequireProxy(url string, contextType configuration.ContextType) {
	independent.RequiredProxies.Set(url, contextType)
}

// Pipe the controller to the proxy
func (independent *Service) Pipe(proxyUrl string, name string) error {
	validProxy := false
	for url := range independent.RequiredProxies {
		if strings.Compare(url, proxyUrl) == 0 {
			validProxy = true
			break
		}
	}
	if !validProxy {
		return fmt.Errorf("proxy '%s' url not required. call independent.RequireProxy", proxyUrl)
	}

	if err := independent.Controllers.Exist(name); err != nil {
		return fmt.Errorf("controller instance '%s' not added. call independent.AddController: %w", name, err)
	}

	independent.Pipelines.Set(proxyUrl, name)

	return nil
}

// returns the extension urls
func (independent *Service) requiredControllerExtensions() []string {
	var extensions []string
	for _, controllerInterface := range independent.Controllers {
		c := controllerInterface.(controller.Interface)
		extensions = append(extensions, c.RequiredExtensions()...)
	}

	return extensions
}

func (independent *Service) prepareServiceConfiguration(expectedType configuration.ServiceType) error {
	// validate the independent itself
	config := independent.Config
	serviceConfig := independent.Config.Service
	if len(serviceConfig.Type) == 0 {
		if !argument.Exist(argument.Url) {
			return fmt.Errorf("missing --url")
		}

		url, err := argument.Value(argument.Url)
		if err != nil {
			return fmt.Errorf("argument.Value: %w", err)
		}

		serviceConfig = configuration.Service{
			Type:      expectedType,
			Url:       url,
			Instance:  config.Name + " 1",
			Pipelines: key_value.Empty(),
		}
	} else if serviceConfig.Type != expectedType {
		return fmt.Errorf("service type is overwritten. expected '%s', not '%s'", expectedType, serviceConfig.Type)
	}

	independent.Config.Service = serviceConfig
	independent.Config.Context.SetUrl(serviceConfig.Url)

	return nil
}

func (independent *Service) prepareControllerConfigurations() error {
	// validate the Controllers
	for name, controllerInterface := range independent.Controllers {
		c := controllerInterface.(controller.Interface)

		err := independent.PrepareControllerConfiguration(name, c.ControllerType())
		if err != nil {
			return fmt.Errorf("prepare '%s' controller configuration as '%s' type: %w", name, c.ControllerType(), err)
		}
	}

	return nil
}

func (independent *Service) PrepareControllerConfiguration(name string, as configuration.Type) error {
	serviceConfig := independent.Config.Service

	// validate the Controllers
	controllerConfig, err := serviceConfig.GetController(name)
	if err == nil {
		if controllerConfig.Type != as {
			return fmt.Errorf("controller expected to be of '%s' type, not '%s'", as, controllerConfig.Type)
		}
	} else {
		controllerConfig = configuration.Controller{
			Type: as,
			Name: name,
		}

		serviceConfig.Controllers = append(serviceConfig.Controllers, controllerConfig)
		independent.Config.Service = serviceConfig
	}

	err = independent.prepareInstanceConfiguration(controllerConfig)
	if err != nil {
		return fmt.Errorf("failed preparing '%s' controller instance configuration: %w", controllerConfig.Name, err)
	}

	return nil
}

func (independent *Service) prepareInstanceConfiguration(controllerConfig configuration.Controller) error {
	serviceConfig := independent.Config.Service

	if len(controllerConfig.Instances) == 0 {
		port := independent.Config.GetFreePort()

		sourceInstance := configuration.ControllerInstance{
			Name:     controllerConfig.Name,
			Instance: controllerConfig.Name + "1",
			Port:     uint64(port),
		}
		controllerConfig.Instances = append(controllerConfig.Instances, sourceInstance)
		serviceConfig.SetController(controllerConfig)
		independent.Config.Service = serviceConfig
	} else {
		if controllerConfig.Instances[0].Port == 0 {
			return fmt.Errorf("the port should not be 0 in the source")
		}
	}

	return nil
}

// prepareConfiguration prepares yaml in service, controller, and controller instances
func (independent *Service) prepareConfiguration(expectedType configuration.ServiceType) error {
	if err := independent.prepareServiceConfiguration(expectedType); err != nil {
		return fmt.Errorf("prepareServiceConfiguration as %s: %w", expectedType, err)
	}

	// validate the Controllers
	if err := independent.prepareControllerConfigurations(); err != nil {
		return fmt.Errorf("prepareControllerConfigurations: %w", err)
	}

	return nil
}

// preparePipelineConfiguration checks that proxy url and controllerName are valid.
// Then, in the Config, it makes sure that dependency is linted.
func (independent *Service) preparePipelineConfiguration(dep *dev.Dep, controllerName string) error {
	proxyUrl := dep.Url()
	found := false
	for requiredProxy := range independent.RequiredProxies {
		if strings.Compare(proxyUrl, requiredProxy) == 0 {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("proxy '%s' not found. add using independent.RequireProxy()", proxyUrl)
	}

	if err := independent.Controllers.Exist(controllerName); err != nil {
		return fmt.Errorf("independent.Controllers.Exist of '%s': %w", controllerName, err)
	}

	err := preparePipelineConfiguration(independent.Config, dep, controllerName, independent.Logger)

	if err != nil {
		return fmt.Errorf("service.preparePipelineConfiguration: %w", err)
	}

	// pipelines from the configurations are not used.
	// are they necessary?
	independent.Config.Service.SetPipeline(proxyUrl, controllerName)

	return nil
}

// onClose closing all the dependencies in the context.
func (independent *Service) onClose(request message.Request, logger *log.Logger, _ ...*remote.ClientSocket) message.Reply {
	logger.Info("service received a signal to close",
		"service", independent.Config.Service.Url,
		"todo", "close all controllers",
	)

	for name, controllerInterface := range independent.Controllers {
		c := controllerInterface.(controller.Interface)
		if c == nil {
			continue
		}

		// I expect that killing the process will release its resources as well.
		err := c.Close()
		if err != nil {
			logger.Error("controller.Close", "error", err, "controller", name)
			request.Fail(fmt.Sprintf(`controller.Close("%s"): %v`, name, err))
		}
		logger.Info("controller was closed", "name", name)
	}

	// remove the context lint
	independent.Context = nil

	logger.Info("all controllers in the service were closed")
	return request.Ok(key_value.Empty())
}

// Run the context in the background. If it failed to run, then return an error.
// The url parameter is the main service to which this context belongs too.
//
// The logger is the server logger as is. The context will create its own logger from it.
func (independent *Service) runManager() error {
	replier, err := controller.SyncReplier(independent.Logger.Child("manager"))
	if err != nil {
		return fmt.Errorf("controller.SyncReplier: %w", err)
	}

	config := configuration.InternalConfiguration(configuration.ManagerName(independent.Config.Service.Url))
	replier.AddConfig(config, independent.Config.Service.Url)

	closeRoute := command.NewRoute("close", independent.onClose)
	err = replier.AddRoute(closeRoute)
	if err != nil {
		return fmt.Errorf(`replier.AddRoute("close"): %w`, err)
	}

	independent.manager = replier
	go independent.manager.Run()

	return nil
}

// Prepare the services by validating, linting the configurations, as well as setting up the dependencies
func (independent *Service) Prepare(as configuration.ServiceType) error {
	if len(independent.Controllers) == 0 {
		return fmt.Errorf("no Controllers. call independent.AddController")
	}


	//
	// prepare the configuration with the service, it's controllers and instances.
	// it doesn't prepare the proxies, pipelines and extensions
	//----------------------------------------------------
	err = independent.prepareConfiguration(as)
	if err != nil {
		return fmt.Errorf("prepareConfiguration: %w", err)
	}

	//
	// prepare the context for dependencies
	//---------------------------------------------------
	independent.Context, err = prepareContext(independent.Config.Context)
	if err != nil {
		return fmt.Errorf("service.prepareContext: %w", err)
	}

	requiredExtensions := independent.requiredControllerExtensions()

	err = context.Run(independent.Config.Service.Url, independent.Logger)
	if err != nil {
		return fmt.Errorf("context.Run: %w", err)
	}

	//
	// prepare proxies configurations
	//--------------------------------------------------
	if len(independent.RequiredProxies) > 0 {
		for requiredProxy, contextInterface := range independent.RequiredProxies {
			contextType := contextInterface.(configuration.ContextType)
			var dep *dev.Dep

			dep, err = independent.Context.New(requiredProxy)
			if err != nil {
				err = fmt.Errorf(`independent.Context.New("%s"): %w`, requiredProxy, err)
				goto closeContext
			}

			if err = independent.prepareProxyConfiguration(dep, contextType); err != nil {
				err = fmt.Errorf("service.prepareProxyConfiguration of %s in context %s: %w", requiredProxy, contextType, err)
				goto closeContext
			}
		}

		if len(independent.Pipelines) == 0 {
			err = fmt.Errorf("no pipepline to lint the proxy to the controller")
			goto closeContext
		}

		for requiredProxy, controllerInterface := range independent.Pipelines {
			controllerName := controllerInterface.(string)
			var dep *dev.Dep

			dep, err = independent.Context.Dep(requiredProxy)
			if err != nil {
				err = fmt.Errorf(`independent.Context.Dep("%s"): %w`, requiredProxy, err)
				goto closeContext
			}

			if err = independent.preparePipelineConfiguration(dep, controllerName); err != nil {
				err = fmt.Errorf("preparePipelineConfiguration '%s'=>'%s': %w", requiredProxy, controllerName, err)
				goto closeContext
			}
		}
	}

	//
	// prepare extensions configurations
	//------------------------------------------------------
	if len(requiredExtensions) > 0 {
		independent.Logger.Warn("extensions needed to be prepared", "extensions", requiredExtensions)
		for _, requiredExtension := range requiredExtensions {
			var dep *dev.Dep

			dep, err = independent.Context.New(requiredExtension)
			if err != nil {
				err = fmt.Errorf(`independent.Context.New("%s"): %w`, requiredExtension, err)
				goto closeContext
			}

			if err = independent.prepareExtensionConfiguration(dep); err != nil {
				err = fmt.Errorf(`service.prepareExtensionConfiguration("%s"): %w`, requiredExtension, err)
				goto closeContext
			}
		}
	}

	//
	// lint extensions, configurations to the controllers
	//---------------------------------------------------------
	for name, controllerInterface := range independent.Controllers {
		c := controllerInterface.(controller.Interface)
		var controllerConfig configuration.Controller
		var controllerExtensions []string

		controllerConfig, err = independent.Config.Service.GetController(name)
		if err != nil {
			err = fmt.Errorf("c '%s' registered in the service, no config found: %w", name, err)
			goto closeContext
		}

		c.AddConfig(&controllerConfig, independent.Config.Service.Url)
		controllerExtensions = c.RequiredExtensions()
		for _, extensionUrl := range controllerExtensions {
			requiredExtension := independent.Config.Service.GetExtension(extensionUrl)
			c.AddExtensionConfig(requiredExtension)
		}
	}

	// run proxies if they are needed.
	if len(independent.RequiredProxies) > 0 {
		for requiredProxy := range independent.RequiredProxies {
			// We don't check for the error, since preparing the configuration should do that already.
			dep, _ := independent.Context.Dep(requiredProxy)

			if err = independent.prepareProxy(dep); err != nil {
				err = fmt.Errorf(`service.prepareProxy("%s"): %w`, requiredProxy, err)
				goto closeContext
			}
		}
	}

	// run extensions if they are needed.
	if len(requiredExtensions) > 0 {
		for _, requiredExtension := range requiredExtensions {
			// We don't check for the error, since preparing the configuration should do that already.
			dep, _ := independent.Context.Dep(requiredExtension)

			if err = independent.prepareExtension(dep); err != nil {
				err = fmt.Errorf(`service.prepareExtension("%s"): %w`, requiredExtension, err)
				goto closeContext
			}
		}
	}

	return nil

	// error happened, close the context
closeContext:
	if err == nil {
		return fmt.Errorf("error is expected, it doesn't exist though")
	}
	return err
}

// BuildConfiguration is invoked from Run. It's passed if the --build-configuration flag was given.
// This function creates a yaml configuration with the service parameters.
func (independent *Service) BuildConfiguration() {
	if !argument.Exist(argument.BuildConfiguration) {
		return
	}
	relativePath, err := argument.Value(argument.Path)
	if err != nil {
		independent.Logger.Fatal("requires 'path' flag", "error", err)
	}

	url, err := argument.Value(argument.Url)
	if err != nil {
		independent.Logger.Fatal("requires 'url' flag", "error", err)
	}

	execPath, err := path.GetExecPath()
	if err != nil {
		independent.Logger.Fatal("path.GetExecPath", "error", err)
	}

	outputPath := path.GetPath(execPath, relativePath)

	independent.Config.Service.Url = url

	err = configuration.WriteService(outputPath, independent.Config.Service)
	if err != nil {
		independent.Logger.Fatal("failed to write the proxy into the file", "error", err)
	}

	independent.Logger.Info("yaml configuration was generated", "output path", outputPath)

	os.Exit(0)
}

// Run the independent service.
func (independent *Service) Run() {
	independent.BuildConfiguration()
	var wg sync.WaitGroup

	err := independent.runManager()
	if err != nil {
		err = fmt.Errorf("independent.runManager: %w", err)
		goto errOccurred
	}

	for name, controllerInterface := range independent.Controllers {
		c := controllerInterface.(controller.Interface)
		if err = independent.Controllers.Exist(name); err != nil {
			independent.Logger.Error("independent.Controllers.Exist", "configuration", name, "error", err)
			break
		}

		wg.Add(1)
		go func() {
			err = c.Run()
			wg.Done()

		}()
	}

	err = independent.Context.ServiceReady(independent.Logger)
	if err != nil {
		goto errOccurred
	}

	wg.Wait()

errOccurred:
	if err != nil {
		if independent.Context != nil {
			independent.Logger.Warn("context wasn't closed, close it")
			independent.Logger.Warn("might happen a race condition." +
				"if the error occurred in the controller" +
				"here we will close the context." +
				"context will close the service." +
				"service will again will come to this place, since all controllers will be cleaned out" +
				"and controller empty will come to here, it will try to close context again",
			)
			closeErr := independent.Context.Close(independent.Logger)
			if closeErr != nil {
				independent.Logger.Fatal("independent.Context.Close", "error", closeErr, "error to print", err)
			}
		}

		independent.Logger.Fatal("one or more controllers removed, exiting from service", "error", err)
	}
}

func prepareContext(config *configuration.Context) (*dev.Context, error) {
	// get the extensions
	context, err := dev.New(config)
	if err != nil {
		return nil, fmt.Errorf("dev.New: %w", err)
	}

	return context, nil
}

// prepareProxy links the proxy with the dependency.
//
// if dependency doesn't exist, it will be downloaded
func (independent *Service) prepareProxy(dep *dev.Dep) error {
	proxyConfiguration := independent.Config.Service.GetProxy(dep.Url())

	independent.Logger.Info("prepare proxy", "url", proxyConfiguration.Url, "port", proxyConfiguration.Port)
	err := dep.Prepare(proxyConfiguration.Port, independent.Logger)
	if err != nil {
		return fmt.Errorf(`dep.Prepare("%s"): %w`, dep.Url(), err)
	}

	return nil
}

// prepareExtension links the extension with the dependency.
//
// if dependency doesn't exist, it will be downloaded
func (independent *Service) prepareExtension(dep *dev.Dep) error {
	extensionConfiguration := independent.Config.Service.GetExtension(dep.Url())

	independent.Logger.Info("prepare extension", "url", extensionConfiguration.Url, "port", extensionConfiguration.Port)
	err := dep.Prepare(extensionConfiguration.Port, independent.Logger)
	if err != nil {
		return fmt.Errorf(`dep.Prepare("%s"): %w`, dep.Url(), err)
	}
	return nil
}

// prepareProxyConfiguration links the proxy with the dependency.
//
// if dependency doesn't exist, it will be downloaded
func (independent *Service) prepareProxyConfiguration(dep *dev.Dep, proxyContext configuration.ContextType) error {
	err := dep.PrepareConfiguration(independent.Logger)
	if err != nil {
		return fmt.Errorf("dev.PrepareConfiguration on %s: %w", dep.Url(), err)
	}

	service, err := dep.Configuration()
	converted, err := configuration.ServiceToProxy(&service, proxyContext)
	if err != nil {
		return fmt.Errorf("configuration.ServiceToProxy: %w", err)
	}

	proxyConfiguration := independent.Config.Service.GetProxy(dep.Url())
	if proxyConfiguration == nil {
		independent.Config.Service.SetProxy(converted)
	} else {
		if strings.Compare(proxyConfiguration.Url, converted.Url) != 0 {
			return fmt.Errorf("the proxy urls are not matching. in your configuration: %s, in the deps: %s", proxyConfiguration.Url, converted.Url)
		}
		if proxyConfiguration.Context != converted.Context {
			return fmt.Errorf("the proxy contexts are not matching. in your configuration: %s, in the deps: %s", proxyConfiguration.Context, converted.Context)
		}
		if proxyConfiguration.Port != converted.Port {
			independent.Logger.Warn("dependency port not matches to the proxy port. Overwriting the source", "port", proxyConfiguration.Port, "dependency port", converted.Port)

			source, _ := service.GetController(configuration.SourceName)
			source.Instances[0].Port = proxyConfiguration.Port

			service.SetController(source)

			err = dep.SetConfiguration(service)
			if err != nil {
				return fmt.Errorf("failed to update source port in dependency porxy: '%s': %w", dep.Url(), err)
			}
		}
	}

	return nil
}

func (independent *Service) prepareExtensionConfiguration(dep *dev.Dep) error {
	err := dep.PrepareConfiguration(independent.Logger)
	if err != nil {
		return fmt.Errorf("dev.PrepareConfiguration on %s: %w", dep.Url(), err)
	}

	service, err := dep.Configuration()
	converted, err := configuration.ServiceToExtension(&service, independent.Config.Context.Type)
	if err != nil {
		return fmt.Errorf("configuration.ServiceToExtension: %w", err)
	}

	extensionConfiguration := independent.Config.Service.GetExtension(dep.Url())
	if extensionConfiguration == nil {
		independent.Config.Service.SetExtension(converted)
	} else {
		if strings.Compare(extensionConfiguration.Url, converted.Url) != 0 {
			return fmt.Errorf("the extension url in your '%s' configuration not matches to '%s' in the dependency", extensionConfiguration.Url, converted.Url)
		}
		if extensionConfiguration.Port != extensionConfiguration.Port {
			independent.Logger.Warn("dependency port not matches to the extension port. Overwriting the source", "port", extensionConfiguration.Port, "dependency port", converted.Port)

			main, _ := service.GetFirstController()
			main.Instances[0].Port = extensionConfiguration.Port

			service.SetController(main)

			err = dep.SetConfiguration(service)
			if err != nil {
				return fmt.Errorf("failed to update port in dependency extension: '%s': %w", dep.Url(), err)
			}
		}
	}

	return nil
}

// preparePipelineConfiguration checks that proxy url and controllerName are valid.
// Then, in the configuration, it makes sure that dependency is linted.
func preparePipelineConfiguration(config *configuration.Config, dep *dev.Dep, controllerName string, logger *log.Logger) error {
	//
	// lint the dependency proxy's destination to the independent independent's controller
	//--------------------------------------------------
	proxyConfig, err := dep.Configuration()
	if err != nil {
		return fmt.Errorf("dep.Configuration: %w", err)
	}

	destinationConfig, err := proxyConfig.GetController(configuration.DestinationName)
	if err != nil {
		return fmt.Errorf("getting dependency proxy's destination configuration failed: %w", err)
	}

	controllerConfig, err := config.Service.GetController(controllerName)
	if err != nil {
		return fmt.Errorf("getting '%s' controller from independent configuration failed: %w", controllerName, err)
	}

	// somehow it will work with only one instance. but in the future maybe another instances as well.
	destinationInstanceConfig := destinationConfig.Instances[0]
	instanceConfig := controllerConfig.Instances[0]

	if destinationInstanceConfig.Port != instanceConfig.Port {
		logger.Info("the dependency proxy destination not match to the controller",
			"proxy url", dep.Url(),
			"destination port", destinationInstanceConfig.Port,
			"independent controller port", instanceConfig.Port)

		destinationInstanceConfig.Port = instanceConfig.Port
		destinationConfig.Instances[0] = destinationInstanceConfig
		proxyConfig.SetController(destinationConfig)

		logger.Info("linting dependency proxy's destination port", "new port", instanceConfig.Port)
		logger.Warn("todo", 1, "if dependency proxy is running, then it should be restarted")
		err := dep.SetConfiguration(proxyConfig)
		if err != nil {
			return fmt.Errorf("dep.SetConfiguration: %w", err)
		}
	}

	return nil
}
