package service

import (
	"fmt"
	"github.com/ahmetson/service-lib/configuration"
)

// Service type defined in the configuration
type Service struct {
	Type        ServiceType
	Url         string
	Id          string
	Controllers []Controller
	Proxies     []Proxy
	Extensions  []Extension
	Pipelines   []Pipeline
}

// Lint sets the reference to the parent from the child.
//
// If the child configuration is used independently, then
// there is no way to know to which parent it belongs too.
//
// In this case, it sets the reference to the controller from the controller reference.
// If the controller instances are used independently, then other services may know to which service they belong too.
func (s *Service) Lint() error {
	// Lint controller instances to the controllers
	for cI, c := range s.Controllers {
		for iI, instance := range c.Instances {
			if len(instance.ControllerCategory) > 0 {
				if instance.ControllerCategory != c.Category {
					return fmt.Errorf("invalid name for controller instance. "+
						"In service instance '%s', controller '%s', instance '%s'. "+
						"the '%s' name in the controller instance should be '%s'",
						s.Id, c.Category, instance.Id, instance.ControllerCategory, c.Category)
				} else {
					continue
				}
			}

			instance.ControllerCategory = c.Category
			c.Instances[iI] = instance
		}

		s.Controllers[cI] = c
	}

	return nil
}

// ValidateTypes the parameters of the service
func (s *Service) ValidateTypes() error {
	if err := ValidateServiceType(s.Type); err != nil {
		return fmt.Errorf("identity.ValidateServiceType: %v", err)
	}

	for _, c := range s.Controllers {
		if err := ValidateControllerType(c.Type); err != nil {
			return fmt.Errorf("controller.ValidateControllerType: %v", err)
		}
	}

	return nil
}

// GetController returns the controller configuration by the controller name.
// If the controller doesn't exist, then it returns an error.
func (s *Service) GetController(name string) (Controller, error) {
	for _, c := range s.Controllers {
		if c.Category == name {
			return c, nil
		}
	}

	return Controller{}, fmt.Errorf("'%s' controller was not found in '%s' service's configuration", name, s.Url)
}

// GetControllers returns the multiple controllers of the given name.
// If the controllers don't exist, then it returns an error
func (s *Service) GetControllers(name string) ([]*Controller, error) {
	controllers := make([]*Controller, 0, len(s.Controllers))
	count := 0

	for _, c := range s.Controllers {
		if c.Category == name {
			controllers[count] = &c
			count++
		}
	}

	if len(controllers) == 0 {
		return nil, fmt.Errorf("no '%s' controlelr config", name)
	}
	return controllers, nil
}

// GetFirstController returns the controller without requiring its name.
// If the service doesn't have controllers, then it will return an error.
func (s *Service) GetFirstController() (Controller, error) {
	if len(s.Controllers) == 0 {
		return Controller{}, fmt.Errorf("service '%s' doesn't have any controllers in yaml file", s.Url)
	}

	controller := s.Controllers[0]
	return controller, nil
}

// GetExtension returns the extension configuration by the url.
// If the extension doesn't exist, then it returns nil
func (s *Service) GetExtension(url string) *Extension {
	for _, e := range s.Extensions {
		if e.Url == url {
			return &e
		}
	}

	return nil
}

// GetProxy returns the proxy by its url. If it doesn't exist, returns nil
func (s *Service) GetProxy(url string) *Proxy {
	for _, p := range s.Proxies {
		if p.Url == url {
			return &p
		}
	}

	return nil
}

// SetProxy will set a new proxy. If it exists, it will overwrite it
func (s *Service) SetProxy(proxy Proxy) {
	existing := s.GetProxy(proxy.Url)
	if existing == nil {
		s.Proxies = append(s.Proxies, proxy)
	} else {
		*existing = proxy
	}
}

// SetExtension will set a new extension. If it exists, it will overwrite it
func (s *Service) SetExtension(extension Extension) {
	existing := s.GetExtension(extension.Url)
	if existing == nil {
		s.Extensions = append(s.Extensions, extension)
	} else {
		*existing = extension
	}
}

// SetController adds a new controller. If the controller by the same name exists, it will add a new copy.
func (s *Service) SetController(controller Controller) {
	s.Controllers = append(s.Controllers, controller)
}

func (s *Service) SetPipeline(pipeline Pipeline) {
	s.Pipelines = append(s.Pipelines, pipeline)
}

// SourceName of this type should be listed within the controllers in the configuration
const SourceName = "source"

// DestinationName of this type should be listed within the controllers in the configuration
const DestinationName = "destination"

// ServiceToProxy returns the service in the proxy format
// so that it can be used as a proxy by other services.
//
// If the service has another proxy, then it will find it.
func ServiceToProxy(s *Service, contextType configuration.ContextType) (Proxy, error) {
	if s.Type != ProxyType {
		return Proxy{}, fmt.Errorf("only proxy type of service can be converted")
	}

	controllerConfig, err := s.GetController(SourceName)
	if err != nil {
		return Proxy{}, fmt.Errorf("no source controllerConfig: %w", err)
	}

	if len(controllerConfig.Instances) == 0 {
		return Proxy{}, fmt.Errorf("no source instances")
	}

	instance := Instance{
		Id: controllerConfig.Category + " instance 01",
	}

	if len(s.Proxies) == 0 {
		instance.Port = controllerConfig.Instances[0].Port
	} else {
		beginning, err := findPipelineBeginning(s, SourceName, contextType)
		if err != nil {
			return Proxy{}, fmt.Errorf("findPipelineBeginning: %w", err)
		}
		instance.Port = beginning.Instances[0].Port
	}

	converted := Proxy{
		Url:       s.Url,
		Instances: []Instance{instance},
		Context:   contextType,
	}

	return converted, nil
}

// findPipelineBeginning returns the beginning of the pipeline.
// If the contextType is not a default one, then it will search for the specific context type.
func findPipelineBeginning(s *Service, requiredEnd string, contextType configuration.ContextType) (*Proxy, error) {
	for _, pipeline := range s.Pipelines {
		beginning := pipeline.Beginning()
		if !pipeline.HasBeginning() {
			return nil, fmt.Errorf("no pipeline beginning")
		}
		//end, err := s.Pipelines.GetString(beginning)
		//if err != nil {
		//	return nil, fmt.Errorf("pipeline '%s' get the end: %w", beginning, err)
		//}
		//
		//if strings.Compare(end, requiredEnd) != 0 {
		//	continue
		//}

		proxy := s.GetProxy(beginning)
		if proxy == nil {
			return nil, fmt.Errorf("invalid configuration. pipeline '%s' beginning not found in proxy list", beginning)
		}

		if contextType != configuration.DefaultContext {
			if proxy.Context != contextType {
				continue
			} else {
				return proxy, nil
			}
		} else {
			if proxy.Context == configuration.DefaultContext {
				return proxy, nil
			} else {
				continue
			}
		}
	}

	return nil, fmt.Errorf("no pipeline beginning in the context '%s' by '%s' end", contextType, requiredEnd)
}

// HasProxy checks is there any proxy within the context.
// If the context is default, then it will return true for any context
func (s *Service) HasProxy(contextType configuration.ContextType) bool {
	if len(s.Proxies) == 0 {
		return false
	}
	if contextType == configuration.DefaultContext {
		return true
	}

	for _, proxy := range s.Proxies {
		if proxy.Context == contextType {
			return true
		}
	}

	return false
}

// ServiceToExtension returns the service in the proxy format
// so that it can be used as a proxy
func ServiceToExtension(s *Service, contextType configuration.ContextType) (Extension, error) {
	if s.Type != ExtensionType {
		return Extension{}, fmt.Errorf("only proxy type of service can be converted")
	}

	controllerConfig, err := s.GetFirstController()
	if err != nil {
		return Extension{}, fmt.Errorf("no controllerConfig: %w", err)
	}

	if len(controllerConfig.Instances) == 0 {
		return Extension{}, fmt.Errorf("no controller instances")
	}

	converted := Extension{
		Url: s.Url,
		Id:  controllerConfig.Category + " instance 01",
	}

	if !s.HasProxy(contextType) {
		converted.Port = controllerConfig.Instances[0].Port
	} else {
		beginning, err := findPipelineBeginning(s, SourceName, contextType)
		if err != nil {
			return Extension{}, fmt.Errorf("findPipelineBeginning: %w", err)
		}
		converted.Port = beginning.Instances[0].Port
	}

	return converted, nil
}

type Services []Service
