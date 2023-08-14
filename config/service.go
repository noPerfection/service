package config

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/config-lib"
	"github.com/ahmetson/log-lib"
	"github.com/ahmetson/os-lib/arg"
	"github.com/ahmetson/os-lib/path"
	"github.com/ahmetson/service-lib/config/service"
	"github.com/ahmetson/service-lib/config/service/pipeline"
)

// Service type defined in the config
type Service struct {
	Type        Type
	Url         string
	Id          string
	Controllers []*service.Controller
	Proxies     []*service.Proxy
	Extensions  []*service.Extension
	Pipelines   []*pipeline.Pipeline
	engine      *config.Config
}

type Services []Service

func NewService(logger *log.Logger, as Type) (*Service, error) {
	if !arg.Exist(arg.Url) {
		return nil, fmt.Errorf("missing --url")
	}

	url, err := arg.Value(arg.Url)
	if err != nil {
		return nil, fmt.Errorf("arg.Value: %w", err)
	}

	engine, err := config.New(logger)
	if err != nil {
		return nil, fmt.Errorf("config.New: %w", err)
	}

	execPath, err := path.GetExecPath()
	if err != nil {
		return nil, fmt.Errorf("path.GetExecPath: %w", err)
	}

	// Use the service config given from the path
	if arg.Exist(arg.Configuration) {
		configurationPath, err := arg.Value(arg.Configuration)
		if err != nil {
			return nil, fmt.Errorf("failed to get the config path: %w", err)
		}

		absPath := path.GetPath(execPath, configurationPath)

		dir, fileName := path.SplitServicePath(absPath)
		engine.Engine().Set("SERVICE_CONFIG_NAME", fileName)
		engine.Engine().Set("SERVICE_CONFIG_PATH", dir)
	} else {
		engine.Engine().SetDefault("SERVICE_CONFIG_NAME", "service")
		engine.Engine().SetDefault("SERVICE_CONFIG_PATH", execPath)
	}

	configName := engine.Engine().GetString("SERVICE_CONFIG_NAME")
	configPath := engine.Engine().GetString("SERVICE_CONFIG_PATH")
	// load the service config
	engine.Engine().SetConfigName(configName)
	engine.Engine().SetConfigType("yaml")
	engine.Engine().AddConfigPath(configPath)

	serviceConfig, err := engine.ReadFile()
	if err != nil {
		logger.Fatal("config.readFile", "error", err)
	}

	serv, ok := serviceConfig.(*Service)
	if !ok {
		return &Service{
			Type:      as,
			Url:       url,
			Id:        path.FileName(url),
			Pipelines: make([]*pipeline.Pipeline, 0),
			engine:    engine,
		}, nil
	}

	return serv, nil
}

func (s *Service) Parent() *config.Config {
	return s.engine
}

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

// UnmarshalService decodes the yaml into the config.
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
		return nil, fmt.Errorf("failed to convert raw config service to config.Service: %w", err)
	}
	err = serviceConfig.PrepareService()
	if err != nil {
		return nil, fmt.Errorf("prepareService: %w", err)
	}

	return &serviceConfig, nil
}

// Lint sets the reference to the parent from the child.
//
// If the child config is used independently, then
// there is no way to know to which parent it belongs too.
//
// In this case, it sets the reference to the handler from the handler reference.
// If the handler instances are used independently, then other services may know to which service they belong too.
func (s *Service) Lint() error {
	// Lint handler instances to the controllers
	for cI, c := range s.Controllers {
		for iI, instance := range c.Instances {
			if len(instance.ControllerCategory) > 0 {
				if instance.ControllerCategory != c.Category {
					return fmt.Errorf("invalid name for handler instance. "+
						"In service instance '%s', handler '%s', instance '%s'. "+
						"the '%s' name in the handler instance should be '%s'",
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
		if err := service.ValidateControllerType(c.Type); err != nil {
			return fmt.Errorf("handler.ValidateControllerType: %v", err)
		}
	}

	return nil
}

// GetController returns the handler config by the handler name.
// If the handler doesn't exist, then it returns an error.
func (s *Service) GetController(name string) (*service.Controller, error) {
	for _, c := range s.Controllers {
		if c.Category == name {
			return c, nil
		}
	}

	return nil, fmt.Errorf("'%s' handler was not found in '%s' service's config", name, s.Url)
}

// GetControllers returns the multiple controllers of the given name.
// If the controllers don't exist, then it returns an error
func (s *Service) GetControllers(name string) ([]*service.Controller, error) {
	controllers := make([]*service.Controller, 0, len(s.Controllers))
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

// GetFirstController returns the handler without requiring its name.
// If the service doesn't have controllers, then it will return an error.
func (s *Service) GetFirstController() (*service.Controller, error) {
	if len(s.Controllers) == 0 {
		return nil, fmt.Errorf("service '%s' doesn't have any controllers in yaml file", s.Url)
	}

	controller := s.Controllers[0]
	return controller, nil
}

// GetExtension returns the extension config by the url.
// If the extension doesn't exist, then it returns nil
func (s *Service) GetExtension(url string) *service.Extension {
	for _, e := range s.Extensions {
		if e.Url == url {
			return e
		}
	}

	return nil
}

// GetProxy returns the proxy by its url. If it doesn't exist, returns nil
func (s *Service) GetProxy(url string) *service.Proxy {
	for _, p := range s.Proxies {
		if p.Url == url {
			return p
		}
	}

	return nil
}

// SetProxy will set a new proxy. If it exists, it will overwrite it
func (s *Service) SetProxy(proxy *service.Proxy) {
	existing := s.GetProxy(proxy.Url)
	if existing == nil {
		s.Proxies = append(s.Proxies, proxy)
	} else {
		*existing = *proxy
	}
}

// SetExtension will set a new extension. If it exists, it will overwrite it
func (s *Service) SetExtension(extension *service.Extension) {
	existing := s.GetExtension(extension.Url)
	if existing == nil {
		s.Extensions = append(s.Extensions, extension)
	} else {
		*existing = *extension
	}
}

// SetController adds a new handler. If the handler by the same name exists, it will add a new copy.
func (s *Service) SetController(controller *service.Controller) {
	s.Controllers = append(s.Controllers, controller)
}

func (s *Service) SetPipeline(pipeline *pipeline.Pipeline) {
	s.Pipelines = append(s.Pipelines, pipeline)
}

func (s *Service) HasProxy() bool {
	return len(s.Proxies) > 0
}
