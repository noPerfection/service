package service

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/configuration/service/pipeline"
)

// Service type defined in the configuration
type Service struct {
	Type        Type
	Url         string
	Id          string
	Controllers []*Controller
	Proxies     []*Proxy
	Extensions  []*Extension
	Pipelines   []*pipeline.Pipeline
}

type Services []Service

func (s *Service) PrepareService() error {
	err := s.ValidateTypes()
	if err != nil {
		return fmt.Errorf("service.ValidateTypes: %w", err)
	}
	err = s.Lint()
	if err != nil {
		return fmt.Errorf("service.Lint: %w", err)
	}

	return nil
}

// UnmarshalService decodes the yaml into the configuration.
func UnmarshalService(services []interface{}) (*Service, error) {
	if len(services) == 0 {
		return nil, nil
	}

	kv, err := key_value.NewFromInterface(services[0])
	if err != nil {
		return nil, fmt.Errorf("failed to convert raw config service into map: %w", err)
	}

	var serviceConfig Service
	err = kv.Interface(&serviceConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to convert raw config service to configuration.Service: %w", err)
	}
	err = serviceConfig.PrepareService()
	if err != nil {
		return nil, fmt.Errorf("prepareService: %w", err)
	}

	return &serviceConfig, nil
}

// Lint sets the reference to the parent from the child.
//
// If the child configuration is used independently, then
// there is no way to know to which parent it belongs too.
//
// In this case, it sets the reference to the server from the server reference.
// If the server instances are used independently, then other services may know to which service they belong too.
func (s *Service) Lint() error {
	// Lint server instances to the controllers
	for cI, c := range s.Controllers {
		for iI, instance := range c.Instances {
			if len(instance.ControllerCategory) > 0 {
				if instance.ControllerCategory != c.Category {
					return fmt.Errorf("invalid name for server instance. "+
						"In service instance '%s', server '%s', instance '%s'. "+
						"the '%s' name in the server instance should be '%s'",
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
			return fmt.Errorf("server.ValidateControllerType: %v", err)
		}
	}

	return nil
}

// GetController returns the server configuration by the server name.
// If the server doesn't exist, then it returns an error.
func (s *Service) GetController(name string) (*Controller, error) {
	for _, c := range s.Controllers {
		if c.Category == name {
			return c, nil
		}
	}

	return nil, fmt.Errorf("'%s' server was not found in '%s' service's configuration", name, s.Url)
}

// GetControllers returns the multiple controllers of the given name.
// If the controllers don't exist, then it returns an error
func (s *Service) GetControllers(name string) ([]*Controller, error) {
	controllers := make([]*Controller, 0, len(s.Controllers))
	count := 0

	for _, c := range s.Controllers {
		if c.Category == name {
			controllers[count] = c
			count++
		}
	}

	if len(controllers) == 0 {
		return nil, fmt.Errorf("no '%s' controlelr config", name)
	}
	return controllers, nil
}

// GetFirstController returns the server without requiring its name.
// If the service doesn't have controllers, then it will return an error.
func (s *Service) GetFirstController() (*Controller, error) {
	if len(s.Controllers) == 0 {
		return nil, fmt.Errorf("service '%s' doesn't have any controllers in yaml file", s.Url)
	}

	controller := s.Controllers[0]
	return controller, nil
}

// GetExtension returns the extension configuration by the url.
// If the extension doesn't exist, then it returns nil
func (s *Service) GetExtension(url string) *Extension {
	for _, e := range s.Extensions {
		if e.Url == url {
			return e
		}
	}

	return nil
}

// GetProxy returns the proxy by its url. If it doesn't exist, returns nil
func (s *Service) GetProxy(url string) *Proxy {
	for _, p := range s.Proxies {
		if p.Url == url {
			return p
		}
	}

	return nil
}

// SetProxy will set a new proxy. If it exists, it will overwrite it
func (s *Service) SetProxy(proxy *Proxy) {
	existing := s.GetProxy(proxy.Url)
	if existing == nil {
		s.Proxies = append(s.Proxies, proxy)
	} else {
		*existing = *proxy
	}
}

// SetExtension will set a new extension. If it exists, it will overwrite it
func (s *Service) SetExtension(extension *Extension) {
	existing := s.GetExtension(extension.Url)
	if existing == nil {
		s.Extensions = append(s.Extensions, extension)
	} else {
		*existing = *extension
	}
}

// SetController adds a new server. If the server by the same name exists, it will add a new copy.
func (s *Service) SetController(controller *Controller) {
	s.Controllers = append(s.Controllers, controller)
}

func (s *Service) SetPipeline(pipeline *pipeline.Pipeline) {
	s.Pipelines = append(s.Pipelines, pipeline)
}

func (s *Service) HasProxy() bool {
	return len(s.Proxies) > 0
}
