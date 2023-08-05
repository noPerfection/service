// Package independent is the primary service.
// This package is calling out the context. Then within that context sets up
// - controller
// - proxies
// - extensions
package independent

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/communication/command"
	"github.com/ahmetson/service-lib/communication/message"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/configuration/argument"
	"github.com/ahmetson/service-lib/configuration/path"
	"github.com/ahmetson/service-lib/configuration/service"
	"github.com/ahmetson/service-lib/context/dev"
	"github.com/ahmetson/service-lib/controller"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/service-lib/remote"
	"os"
	"strings"
	"sync"
)

// Service keeps all necessary parameters of the service.
type Service struct {
	Config          *configuration.Config
	Controllers     key_value.KeyValue
	pipelines       []service.Pipeline // Pipeline beginning: url => [Pipes]
	RequiredProxies key_value.KeyValue // url => context type
	Logger          *log.Logger
	Context         *dev.Context
	manager         controller.Interface // manage this service from other parts. it should be called before context run
}

// New service with the configuration engine and logger. Logger is used as is.
func New(config *configuration.Config, logger *log.Logger) (*Service, error) {
	independent := Service{
		Config:          config,
		Logger:          logger,
		Controllers:     key_value.Empty(),
		RequiredProxies: key_value.Empty(),
		pipelines:       make([]service.Pipeline, 0),
	}

	return &independent, nil
}

// AddController by their name
func (independent *Service) AddController(name string, controller controller.Interface) {
	independent.Controllers.Set(name, controller)
}

// RequireProxy adds a proxy that's needed for this service to run
func (independent *Service) RequireProxy(url string, contextType configuration.ContextType) {
	independent.RequiredProxies.Set(url, contextType)
}

func (independent *Service) IsProxyRequired(proxyUrl string) bool {
	for url := range independent.RequiredProxies {
		if strings.Compare(url, proxyUrl) == 0 {
			return true
		}
	}

	return false
}

func (independent *Service) GetProxyContext(proxyUrl string) configuration.ContextType {
	contextType, ok := independent.RequiredProxies[proxyUrl].(configuration.ContextType)
	if !ok {
		return configuration.DefaultContext
	}
	return contextType
}

// Pipeline creates a chain of the proxies.
func (independent *Service) Pipeline(pipeEnd *service.PipeEnd, proxyUrls ...string) error {
	if len(proxyUrls) == 0 {
		return fmt.Errorf("no proxy")
	}
	for _, proxyUrl := range proxyUrls {
		if !independent.IsProxyRequired(proxyUrl) {
			return fmt.Errorf("proxy '%s' url not required. call independent.RequireProxy", proxyUrl)
		}
	}

	if pipeEnd.IsController() {
		if err := independent.Controllers.Exist(pipeEnd.Id); err != nil {
			return fmt.Errorf("independent.Controllers.Exist('%s') [call independent.AddController()]: %w", pipeEnd.Id, err)
		}
	} else {
		if service.HasServicePipeline(independent.pipelines) {
			return fmt.Errorf("configuration.HasServicePipeline: service pipeline exists")
		}
	}

	pipeline := pipeEnd.Pipeline(proxyUrls)
	if err := pipeline.ValidateHead(); err != nil {
		return fmt.Errorf("pipeline.ValidateHead: %w", err)
	}
	independent.pipelines = append(independent.pipelines, *pipeline)

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

func (independent *Service) prepareServiceConfiguration(expectedType service.ServiceType) error {
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

		serviceConfig = service.Service{
			Type:      expectedType,
			Url:       url,
			Id:        config.Name + " 1",
			Pipelines: make([]service.Pipeline, 0),
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

func (independent *Service) PrepareControllerConfiguration(name string, as service.Type) error {
	serviceConfig := independent.Config.Service

	// validate the Controllers
	controllerConfig, err := serviceConfig.GetController(name)
	if err == nil {
		if controllerConfig.Type != as {
			return fmt.Errorf("controller expected to be of '%s' type, not '%s'", as, controllerConfig.Type)
		}
	} else {
		controllerConfig = service.Controller{
			Type:     as,
			Category: name,
		}

		serviceConfig.Controllers = append(serviceConfig.Controllers, controllerConfig)
		independent.Config.Service = serviceConfig
	}

	err = independent.prepareInstanceConfiguration(controllerConfig)
	if err != nil {
		return fmt.Errorf("failed preparing '%s' controller instance configuration: %w", controllerConfig.Category, err)
	}

	return nil
}

func (independent *Service) prepareInstanceConfiguration(controllerConfig service.Controller) error {
	serviceConfig := independent.Config.Service

	if len(controllerConfig.Instances) == 0 {
		port := independent.Config.GetFreePort()

		sourceInstance := service.Instance{
			ControllerCategory: controllerConfig.Category,
			Id:                 controllerConfig.Category + "1",
			Port:               uint64(port),
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
func (independent *Service) prepareConfiguration(expectedType service.ServiceType) error {
	if err := independent.prepareServiceConfiguration(expectedType); err != nil {
		return fmt.Errorf("prepareServiceConfiguration as %s: %w", expectedType, err)
	}

	// validate the Controllers
	if err := independent.prepareControllerConfigurations(); err != nil {
		return fmt.Errorf("prepareControllerConfigurations: %w", err)
	}

	return nil
}

// lintPipelineConfiguration checks that proxy url and controllerName are valid.
// Then, in the Config, it makes sure that dependency is linted.
func (independent *Service) preparePipelineConfigurations() error {
	hasService := service.HasServicePipeline(independent.pipelines)
	servicePipeline := service.ServicePipeline(independent.pipelines)
	controllerPipelines := service.ControllerPipelines(independent.pipelines)

	if hasService {
		servicePipeline.End.Url = independent.Config.Service.Url
		independent.Logger.Info("dont forget to update the yaml with the pipeline service end url")
	}

	// lets lint the service's last head's destination to this service
	if hasService {
		proxyUrl := servicePipeline.HeadLast()

		dep, err := independent.Context.Dep(proxyUrl)
		if err != nil {
			return fmt.Errorf(`independent.Context.Dep("%s"): %w`, proxyUrl, err)
		}

		proxyConfig, err := dep.Configuration()
		if err != nil {
			return fmt.Errorf("dep.Configuration: %w", err)
		}

		destinationConfigs, err := proxyConfig.GetControllers(service.DestinationName)
		if err != nil {
			return fmt.Errorf("proxyConfig.GetControllers('%s'): %w", service.DestinationName, err)
		}

		controllerAmount := len(independent.Controllers)
		if len(independent.Config.Service.Controllers) != controllerAmount {
			return fmt.Errorf("configuration has not enough controllers")
		}
		// The service has more controllers or less than in the configuration.
		// Let's rewrite them
		if len(destinationConfigs) != controllerAmount {
			// two times more, source and destination for each controller
			proxyConfig.Controllers = make([]service.Controller, controllerAmount*2)
			set := 0

			// rewrite the destinations in the dependency
			for name, raw := range independent.Controllers {
				c := raw.(controller.Interface)

				// set the source
				instance := service.Instance{
					ControllerCategory: service.SourceName,
					Id:                 fmt.Sprintf("%s 01", name),
					Port:               uint64(independent.Config.GetFreePort()),
				}

				controllerConfig := service.Controller{
					Type:      c.ControllerType(),
					Category:  service.SourceName,
					Instances: []service.Instance{instance},
				}
				proxyConfig.Controllers[set] = controllerConfig
				set++

				origControllerConfig, _ := independent.Config.Service.GetController(name)
				desInstance := service.Instance{
					ControllerCategory: service.DestinationName,
					Id:                 fmt.Sprintf("%s 01", name),
					Port:               origControllerConfig.Instances[0].Port,
				}

				desControllerConfig := service.Controller{
					Type:      c.ControllerType(),
					Category:  service.DestinationName,
					Instances: []service.Instance{desInstance},
				}
				proxyConfig.Controllers[set] = desControllerConfig
				set++
			}

			independent.Logger.Info("make sure that converting service to proxy will convert all destinations to the proxy instances")
			converted, err := service.ServiceToProxy(&proxyConfig, independent.GetProxyContext(proxyUrl))
			if err != nil {
				return fmt.Errorf("failed to convert the proxy")
			}

			independent.Config.Service.SetProxy(converted)
		} else {
			// The order of the destination should match.
			// Check that ports match, if not then update the ports.
			for i, controllerConfig := range independent.Config.Service.Controllers {
				if destinationConfigs[i].Instances[0].Port != controllerConfig.Instances[0].Port {
					independent.Logger.Warn("dependency port not matches to the proxy port. Overwriting the source", "port", controllerConfig.Instances[0].Port, "dependency port", destinationConfigs[i].Instances[0].Port)

					destinationConfigs[i].Instances[0].Port = controllerConfig.Instances[0].Port
				}
			}

			// save the configuration
			for i, controllerConfig := range destinationConfigs {
				proxyConfig.Controllers[i] = *controllerConfig
			}

			err = dep.SetConfiguration(proxyConfig)
			if err != nil {
				return fmt.Errorf("failed to update source port in dependency porxy: '%s': %w", dep.Url(), err)
			}
		}
	}

	var serviceSources []*service.Controller
	if hasService {
		serviceProxyUrl := servicePipeline.Beginning()
		serviceDep, err := independent.Context.Dep(serviceProxyUrl)
		if err != nil {
			return fmt.Errorf(`independent.Context.Dep("%s"): %w`, serviceProxyUrl, err)
		}

		serviceProxyConfig, err := serviceDep.Configuration()
		if err != nil {
			return fmt.Errorf("controllerDep.Configuration: %w", err)
		}

		serviceSources, err = serviceProxyConfig.GetControllers(service.SourceName)
		if err != nil {
			return fmt.Errorf("proxyConfig.GetControllers('%s'): %w", service.SourceName, err)
		}
	}

	// lets lint the controller's last head destination to the service controller's source or
	// to the controller itself.
	for _, pipeline := range controllerPipelines {
		proxyUrl := pipeline.HeadLast()

		if hasService {
			controllerDep, err := independent.Context.Dep(proxyUrl)
			if err != nil {
				return fmt.Errorf(`independent.Context.Dep("%s"): %w`, proxyUrl, err)
			}

			proxyConfig, err := controllerDep.Configuration()
			if err != nil {
				return fmt.Errorf("controllerDep.Configuration: %w", err)
			}

			destinationConfigs, err := proxyConfig.GetControllers(service.DestinationName)
			if err != nil {
				return fmt.Errorf("proxyConfig.GetControllers('%s'): %w", service.DestinationName, err)
			}

			if len(serviceSources) != len(destinationConfigs) {
				proxyConfig.Controllers = make([]service.Controller, len(serviceSources)*2)
				set := 0

				// rewrite the destinations in the dependency
				for _, sourceConfig := range serviceSources {
					// set the source
					instance := service.Instance{
						ControllerCategory: service.SourceName,
						Id:                 fmt.Sprintf("%s source 01", sourceConfig.Instances[0].Id),
						Port:               uint64(independent.Config.GetFreePort()),
					}

					controllerConfig := service.Controller{
						Type:      sourceConfig.Type,
						Category:  service.SourceName,
						Instances: []service.Instance{instance},
					}
					proxyConfig.Controllers[set] = controllerConfig
					set++

					desInstance := service.Instance{
						ControllerCategory: service.DestinationName,
						Id:                 fmt.Sprintf("%s 01", sourceConfig.Instances[0].Id),
						Port:               sourceConfig.Instances[0].Port,
					}

					desControllerConfig := service.Controller{
						Type:      sourceConfig.Type,
						Category:  service.DestinationName,
						Instances: []service.Instance{desInstance},
					}
					proxyConfig.Controllers[set] = desControllerConfig
					set++
				}

				err = controllerDep.SetConfiguration(proxyConfig)
				if err != nil {
					return fmt.Errorf("failed to update source port in dependency porxy: '%s': %w", controllerDep.Url(), err)
				}
			} else {
				// make sure that destination ports are matching to the sources
				// rewrite the destinations in the dependency
				for i, sourceConfig := range serviceSources {
					if sourceConfig.Instances[0].Port != destinationConfigs[i].Instances[0].Port {
						destinationConfigs[i].Instances[0].Port = sourceConfig.Instances[0].Port
					}
				}

				// save the configuration
				for i, controllerConfig := range destinationConfigs {
					proxyConfig.Controllers[i] = *controllerConfig
				}

				err = controllerDep.SetConfiguration(proxyConfig)
				if err != nil {
					return fmt.Errorf("failed to update source port in dependency porxy: '%s': %w", controllerDep.Url(), err)
				}
			}
		} else {
			controllerName := pipeline.End.Id

			proxyConfiguration, _ := independent.Config.Service.GetController(controllerName)

			controllerDep, err := independent.Context.Dep(proxyUrl)
			if err != nil {
				return fmt.Errorf(`independent.Context.Dep("%s"): %w`, proxyUrl, err)
			}

			proxyConfig, err := controllerDep.Configuration()
			if err != nil {
				return fmt.Errorf("controllerDep.Configuration: %w", err)
			}

			sourceConfigs, err := proxyConfig.GetControllers(service.SourceName)
			if err != nil {
				return fmt.Errorf("proxyConfig.GetControllers('%s'): %w", service.DestinationName, err)
			}

			if len(sourceConfigs) > 0 {
				return fmt.Errorf("too many sources, expected only one")
			}

			if proxyConfiguration.Instances[0].Port != sourceConfigs[0].Instances[0].Port {
				independent.Logger.Warn("dependency port not matches to the proxy port. Overwriting the source", "port", proxyConfiguration.Instances[0].Port, "dependency port", proxyConfiguration.Instances[0].Port)

				(*sourceConfigs[0]).Instances[0].Port = proxyConfiguration.Instances[0].Port

				err = controllerDep.SetConfiguration(proxyConfig)
				if err != nil {
					return fmt.Errorf("failed to update source port in dependency porxy: '%s': %w", proxyUrl, err)
				}
			}
		}
	}

	if hasService && servicePipeline.IsMultiHead() {
		// make sure that they link to each other after linting the last head
		proxyUrls := servicePipeline.HeadFront()
		independent.Logger.Info("Make sure that service proxy urls lint to each other", "urls", proxyUrls)

		lastProxyUrl := servicePipeline.HeadLast()

		lastDep, err := independent.Context.Dep(lastProxyUrl)
		if err != nil {
			return fmt.Errorf(`independent.Context.Dep("%s"): %w`, lastProxyUrl, err)
		}

		lastProxyConfig, err := lastDep.Configuration()
		if err != nil {
			return fmt.Errorf("controllerDep.Configuration: %w", err)
		}

		sourceConfigs, err := lastProxyConfig.GetControllers(service.SourceName)
		if err != nil {
			return fmt.Errorf("proxyConfig.GetControllers('%s'): %w", service.DestinationName, err)
		}

		for i := len(proxyUrls) - 1; i >= 0; i-- {
			proxyUrl := proxyUrls[i]
			// if the destinations don't match with the last one, then make sure to rewrite it.
			// otherwise make sure that proxyUrl destination matches with the lastProxyUrl source.
			controllerDep, err := independent.Context.Dep(proxyUrl)
			if err != nil {
				return fmt.Errorf(`independent.Context.Dep("%s"): %w`, proxyUrl, err)
			}

			proxyConfig, err := controllerDep.Configuration()
			if err != nil {
				return fmt.Errorf("controllerDep.Configuration: %w", err)
			}

			destinationConfigs, err := proxyConfig.GetControllers(service.DestinationName)
			if err != nil {
				return fmt.Errorf("proxyConfig.GetControllers('%s'): %w", service.DestinationName, err)
			}

			if len(sourceConfigs) != len(destinationConfigs) {
				proxyConfig.Controllers = make([]service.Controller, len(sourceConfigs)*2)
				set := 0

				// rewrite the destinations in the dependency
				for _, sourceConfig := range sourceConfigs {
					// set the source
					instance := service.Instance{
						ControllerCategory: service.SourceName,
						Id:                 fmt.Sprintf("%s source 01", sourceConfig.Instances[0].Id),
						Port:               uint64(independent.Config.GetFreePort()),
					}

					controllerConfig := service.Controller{
						Type:      sourceConfig.Type,
						Category:  service.SourceName,
						Instances: []service.Instance{instance},
					}
					proxyConfig.Controllers[set] = controllerConfig
					set++

					desInstance := service.Instance{
						ControllerCategory: service.DestinationName,
						Id:                 fmt.Sprintf("%s 01", sourceConfig.Instances[0].Id),
						Port:               sourceConfig.Instances[0].Port,
					}

					desControllerConfig := service.Controller{
						Type:      sourceConfig.Type,
						Category:  service.DestinationName,
						Instances: []service.Instance{desInstance},
					}
					proxyConfig.Controllers[set] = desControllerConfig
					set++
				}

				err = controllerDep.SetConfiguration(proxyConfig)
				if err != nil {
					return fmt.Errorf("failed to update source port in dependency porxy: '%s': %w", controllerDep.Url(), err)
				}
			} else {
				for i, sourceConfig := range sourceConfigs {
					if sourceConfig.Instances[0].Port != destinationConfigs[i].Instances[0].Port {
						destinationConfigs[i].Instances[0].Port = sourceConfig.Instances[0].Port
					}
				}

				// save the configuration
				for i, controllerConfig := range destinationConfigs {
					proxyConfig.Controllers[i] = *controllerConfig
				}

				err := controllerDep.SetConfiguration(proxyConfig)
				if err != nil {
					return fmt.Errorf("failed to update source port in dependency porxy: '%s': %w", controllerDep.Url(), err)
				}
			}

			lastProxyUrl = proxyUrl
			lastDep = controllerDep
			lastProxyConfig = proxyConfig
			sourceConfigs, _ = lastProxyConfig.GetControllers(service.SourceName)
		}
	}

	for _, pipeline := range controllerPipelines {
		if !pipeline.IsMultiHead() {
			continue
		}

		// make sure that they link to each other after linting the last head
		proxyUrls := pipeline.HeadFront()
		independent.Logger.Info("Make sure that controller proxy urls lint to each other", "urls", proxyUrls)

		lastProxyUrl := pipeline.HeadLast()

		lastDep, err := independent.Context.Dep(lastProxyUrl)
		if err != nil {
			return fmt.Errorf(`independent.Context.Dep("%s"): %w`, lastProxyUrl, err)
		}

		lastProxyConfig, err := lastDep.Configuration()
		if err != nil {
			return fmt.Errorf("controllerDep.Configuration: %w", err)
		}

		sourceConfigs, err := lastProxyConfig.GetControllers(service.SourceName)
		if err != nil {
			return fmt.Errorf("proxyConfig.GetControllers('%s'): %w", service.DestinationName, err)
		}

		for i := len(proxyUrls) - 1; i >= 0; i-- {
			proxyUrl := proxyUrls[i]
			// if the destinations don't match with the last one, then make sure to rewrite it.
			// otherwise make sure that proxyUrl destination matches with the lastProxyUrl source.
			controllerDep, err := independent.Context.Dep(proxyUrl)
			if err != nil {
				return fmt.Errorf(`independent.Context.Dep("%s"): %w`, proxyUrl, err)
			}

			proxyConfig, err := controllerDep.Configuration()
			if err != nil {
				return fmt.Errorf("controllerDep.Configuration: %w", err)
			}

			destinationConfigs, err := proxyConfig.GetControllers(service.DestinationName)
			if err != nil {
				return fmt.Errorf("proxyConfig.GetControllers('%s'): %w", service.DestinationName, err)
			}

			if len(sourceConfigs) != len(destinationConfigs) {
				proxyConfig.Controllers = make([]service.Controller, len(sourceConfigs)*2)
				set := 0

				// rewrite the destinations in the dependency
				for _, sourceConfig := range sourceConfigs {
					// set the source
					instance := service.Instance{
						ControllerCategory: service.SourceName,
						Id:                 fmt.Sprintf("%s source 01", sourceConfig.Instances[0].Id),
						Port:               uint64(independent.Config.GetFreePort()),
					}

					controllerConfig := service.Controller{
						Type:      sourceConfig.Type,
						Category:  service.SourceName,
						Instances: []service.Instance{instance},
					}
					proxyConfig.Controllers[set] = controllerConfig
					set++

					desInstance := service.Instance{
						ControllerCategory: service.DestinationName,
						Id:                 fmt.Sprintf("%s 01", sourceConfig.Instances[0].Id),
						Port:               sourceConfig.Instances[0].Port,
					}

					desControllerConfig := service.Controller{
						Type:      sourceConfig.Type,
						Category:  service.DestinationName,
						Instances: []service.Instance{desInstance},
					}
					proxyConfig.Controllers[set] = desControllerConfig
					set++
				}

				err = controllerDep.SetConfiguration(proxyConfig)
				if err != nil {
					return fmt.Errorf("failed to update source port in dependency porxy: '%s': %w", controllerDep.Url(), err)
				}
			} else {
				for i, sourceConfig := range sourceConfigs {
					if sourceConfig.Instances[0].Port != destinationConfigs[i].Instances[0].Port {
						destinationConfigs[i].Instances[0].Port = sourceConfig.Instances[0].Port
					}
				}

				// save the configuration
				for i, controllerConfig := range destinationConfigs {
					proxyConfig.Controllers[i] = *controllerConfig
				}

				err := controllerDep.SetConfiguration(proxyConfig)
				if err != nil {
					return fmt.Errorf("failed to update source port in dependency porxy: '%s': %w", controllerDep.Url(), err)
				}
			}

			lastProxyUrl = proxyUrl
			lastDep = controllerDep
			lastProxyConfig = proxyConfig
			sourceConfigs, _ = lastProxyConfig.GetControllers(service.SourceName)
		}
	}

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
		return fmt.Errorf("controller.SyncReplierType: %w", err)
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
func (independent *Service) Prepare(as service.ServiceType) error {
	if len(independent.Controllers) == 0 {
		return fmt.Errorf("no Controllers. call independent.AddController")
	}

	//
	// prepare the configuration with the service, it's controllers and instances.
	// it doesn't prepare the proxies, pipelines and extensions
	//----------------------------------------------------
	err := independent.prepareConfiguration(as)
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

	err = independent.Context.Run(independent.Logger)
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

			// Sets the default values.
			if err = independent.prepareProxyConfiguration(dep, contextType); err != nil {
				err = fmt.Errorf("service.prepareProxyConfiguration of %s in context %s: %w", requiredProxy, contextType, err)
				goto closeContext
			}
		}

		if len(independent.pipelines) == 0 {
			err = fmt.Errorf("no pipepline to lint the proxy to the controller")
			goto closeContext
		}

		if err = independent.preparePipelineConfigurations(); err != nil {
			err = fmt.Errorf("preparePipelineConfigurations: %w", err)
			goto closeContext
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
		var controllerConfig service.Controller
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

	independent.Logger.Info("prepare proxy", "url", proxyConfiguration.Url, "port", proxyConfiguration.Instances[0].Port)
	err := dep.Prepare(proxyConfiguration.Instances[0].Port, independent.Logger)
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

	depConfig, err := dep.Configuration()
	converted, err := service.ServiceToProxy(&depConfig, proxyContext)
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
		if proxyConfiguration.Instances[0].Port != converted.Instances[0].Port {
			independent.Logger.Warn("dependency port not matches to the proxy port. Overwriting the source", "port", proxyConfiguration.Instances[0].Port, "dependency port", converted.Instances[0].Port)

			source, _ := depConfig.GetController(service.SourceName)
			source.Instances[0].Port = proxyConfiguration.Instances[0].Port

			depConfig.SetController(source)

			err = dep.SetConfiguration(depConfig)
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

	depConfig, err := dep.Configuration()
	converted, err := service.ServiceToExtension(&depConfig, independent.Config.Context.Type)
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
		if extensionConfiguration.Port != converted.Port {
			independent.Logger.Warn("dependency port not matches to the extension port. Overwriting the source", "port", extensionConfiguration.Port, "dependency port", converted.Port)

			main, _ := depConfig.GetFirstController()
			main.Instances[0].Port = extensionConfiguration.Port

			depConfig.SetController(main)

			err = dep.SetConfiguration(depConfig)
			if err != nil {
				return fmt.Errorf("failed to update port in dependency extension: '%s': %w", dep.Url(), err)
			}
		}
	}

	return nil
}
