/*Package independent is used to scaffold the independent service
 */
package independent

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/configuration/argument"
	"github.com/ahmetson/service-lib/controller"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/service-lib/service"
	"os"
	"strings"
	"sync"
)

// Service is the collection of the various Controllers
type Service struct {
	Config          *configuration.Config
	Controllers     key_value.KeyValue
	Pipelines       key_value.KeyValue
	RequiredProxies []string
	Logger          *log.Logger
}

// New service based on the configurations
func New(config *configuration.Config, logger *log.Logger) (*Service, error) {
	independent := Service{
		Config:          config,
		Logger:          logger,
		Controllers:     key_value.Empty(),
		RequiredProxies: []string{},
		Pipelines:       key_value.Empty(),
	}

	return &independent, nil
}

// AddController by their instance name
func (independent *Service) AddController(name string, controller controller.Interface) {
	independent.Controllers.Set(name, controller)
}

func (independent *Service) RequireProxy(url string) {
	independent.RequiredProxies = append(independent.RequiredProxies, url)
}

// Pipe the controller to the proxy
func (independent *Service) Pipe(proxyUrl string, name string) error {
	validProxy := false
	for _, url := range independent.RequiredProxies {
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
		exePath, err := configuration.GetCurrentPath()
		if err != nil {
			return fmt.Errorf("failed to get current executable path: %w", err)
		}

		serviceConfig = configuration.Service{
			Type:     expectedType,
			Url:      exePath,
			Instance: config.Name + " 1",
		}
	} else if serviceConfig.Type != expectedType {
		return fmt.Errorf("service type is overwritten. expected '%s', not '%s'", expectedType, serviceConfig.Type)
	}

	independent.Config.Service = serviceConfig

	return nil
}

func (independent *Service) prepareControllerConfigurations() error {
	serviceConfig := independent.Config.Service

	// validate the Controllers
	for name, controllerInterface := range independent.Controllers {
		c := controllerInterface.(controller.Interface)

		err := independent.PrepareControllerConfiguration(name, c.ControllerType())
		if err == nil {
			return fmt.Errorf("prepare '%s' controller configuration as '%s' type: %w", name, c.ControllerType(), err)
		}
	}

	independent.Config.Service = serviceConfig
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
		controllerConfig := configuration.Controller{
			Type: as,
			Name: name,
		}

		serviceConfig.Controllers = append(serviceConfig.Controllers, controllerConfig)
	}

	err = independent.prepareInstanceConfiguration(controllerConfig)
	if err != nil {
		return fmt.Errorf("failed preparing '%s' controller instance configuration: %w", controllerConfig.Name, err)
	}

	independent.Config.Service = serviceConfig
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
	} else {
		if controllerConfig.Instances[0].Port == 0 {
			return fmt.Errorf("the port should not be 0 in the source")
		}
	}

	independent.Config.Service = serviceConfig
	return nil
}

// prepareConfiguration prepares yaml in service, controller, and controller instances
func (independent *Service) prepareConfiguration(expectedType configuration.ServiceType) error {
	if err := independent.prepareServiceConfiguration(expectedType); err != nil {
		return fmt.Errorf("prepareServiceConfiguration as %s: %w", expectedType, err)
	}
	serviceConfig := independent.Config.Service

	// validate the Controllers
	if err := independent.prepareControllerConfigurations(); err != nil {
		return fmt.Errorf("prepareControllerConfigurations: %w", err)
	}

	// todo validate the extensions

	independent.Config.Service = serviceConfig

	return nil
}

// preparePipelineConfiguration checks that proxy url and controllerName are valid.
// Then, in the Config, it makes sure that dependency is linted.
func (independent *Service) preparePipelineConfiguration(proxyUrl string, controllerName string) error {
	independent.Logger.Info("prepare the pipeline")

	found := false
	for _, requiredProxy := range independent.RequiredProxies {
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

	err := service.PreparePipelineConfiguration(independent.Config, proxyUrl, controllerName, independent.Logger)

	if err != nil {
		return fmt.Errorf("service.PreparePipelineConfiguration: %w", err)
	}
	return nil
}

// Prepare the services by validating, linting the configurations, as well as setting up the dependencies
func (independent *Service) Prepare(as configuration.ServiceType) error {
	if len(independent.Controllers) == 0 {
		return fmt.Errorf("no Controllers. call independent.AddController")
	}

	//
	// prepare the context for dependencies
	//---------------------------------------------------
	err := service.PrepareContext(independent.Config.Context)
	if err != nil {
		return fmt.Errorf("service.PrepareContext: %w", err)
	}

	//
	// prepare the configuration
	//----------------------------------------------------
	err = independent.prepareConfiguration(as)
	if err != nil {
		return fmt.Errorf("prepareConfiguration: %w", err)
	}

	//
	// prepare proxies
	//--------------------------------------------------
	if len(independent.RequiredProxies) > 0 {
		independent.Logger.Info("there are some proxies to setup")
		for _, requiredProxy := range independent.RequiredProxies {
			if err := service.PrepareProxyConfiguration(requiredProxy, independent.Config, independent.Logger); err != nil {
				return fmt.Errorf("service.PrepareProxyConfiguration of %s: %w", requiredProxy, err)
			}
		}

		if len(independent.Pipelines) == 0 {
			return fmt.Errorf("no pipepline to lint the proxy to the controller")
		}

		for requiredProxy, controllerInterface := range independent.Pipelines {
			controllerName := controllerInterface.(string)
			if err := independent.preparePipelineConfiguration(requiredProxy, controllerName); err != nil {
				return fmt.Errorf("preparePipelineConfiguration '%s'=>'%s': %w", requiredProxy, controllerName, err)
			}
		}
	}

	//
	// prepare extensions
	//------------------------------------------------------
	requiredExtensions := independent.requiredControllerExtensions()
	if len(requiredExtensions) > 0 {
		independent.Logger.Warn("extensions needed to be prepared", "extensions", requiredExtensions)
	} else {
		independent.Logger.Info("no extensions needed")
	}

	//
	// lint extensions, configurations to the controllers
	//---------------------------------------------------------
	for name, controllerInterface := range independent.Controllers {
		controller := controllerInterface.(controller.Interface)

		controllerConfig, err := independent.Config.Service.GetController(name)
		if err != nil {
			return fmt.Errorf("controller '%s' registered in the service, no config found: %w", name, err)
		}

		controller.AddConfig(&controllerConfig)
		requiredExtensions := controller.RequiredExtensions()
		for _, extensionUrl := range requiredExtensions {
			requiredExtension := independent.Config.Service.GetExtension(extensionUrl)
			controller.AddExtensionConfig(requiredExtension)
		}
	}

	return nil
}

// BuildConfiguration creates a yaml configuration with the service parameters
func (independent *Service) BuildConfiguration() {
	path, err := argument.Value(argument.Path)
	if err != nil {
		independent.Logger.Fatal("requires 'path' flag", "error", err)
	}

	url, err := argument.Value(argument.Url)
	if err != nil {
		independent.Logger.Fatal("requires 'url' flag", "error", err)
	}

	independent.Config.Service.Url = url

	err = independent.Config.WriteService(path)
	if err != nil {
		independent.Logger.Fatal("failed to write the proxy into the file", "error", err)
	}

	independent.Logger.Info("the proxy was generated", "path", path)

	os.Exit(0)
}

// Run the independent service.
func (independent *Service) Run() {
	if argument.Exist(argument.BuildConfiguration) {
		independent.BuildConfiguration()
	}

	var wg sync.WaitGroup

	for name, controllerInterface := range independent.Controllers {
		c := controllerInterface.(controller.Interface)
		if err := independent.Controllers.Exist(name); err != nil {
			independent.Logger.Fatal("controller configuration not found", "configuration", name, "error", err)
			continue
		}

		wg.Add(1)
		go func() {
			err := c.Run()
			wg.Done()
			if err != nil {
				independent.Logger.Fatal("failed to run the controller", "error", err)
			}
		}()
	}
	wg.Wait()
}
