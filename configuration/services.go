package configuration

import (
	"fmt"
)

type ControllerInstance struct {
	Port     uint64
	Instance string
	Name     string
}

type Controller struct {
	Type      Type
	Name      string
	Instances []ControllerInstance
}

type Proxy struct {
	Name string
	Port uint64
}

type Extension struct {
	Name string
	Port uint64
}

// Service type defined in the configuration
type Service struct {
	Type        ServiceType
	Name        string
	Instance    string
	Controllers []Controller
	Proxies     []Proxy
	Extensions  []Extension
	Pipelines   []string
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

// NewInternalExtension returns the extension that is on another thread, but not on remote.
// The extension will be connected using the inproc protocol, not over TCP.
func NewInternalExtension(name string) *Extension {
	return &Extension{Name: name, Port: 0}
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
			if len(instance.Name) > 0 {
				if instance.Name != c.Name {
					return fmt.Errorf("invalid name for controller instance. "+
						"In service instance '%s', controller '%s', instance '%s'. "+
						"the '%s' name in the controller instance should be '%s'",
						s.Instance, c.Name, instance.Instance, instance.Name, c.Name)
				} else {
					continue
				}
			}

			instance.Name = c.Name
			c.Instances[iI] = instance
		}

		s.Controllers[cI] = c
	}

	return nil
}

// GetController returns the controller configuration by the controller name.
// If the controller doesn't exist, then it returns an error.
func (s *Service) GetController(name string) (Controller, error) {
	for _, c := range s.Controllers {
		if c.Name == name {
			return c, nil
		}
	}

	return Controller{}, fmt.Errorf("'%s' controller was not found in '%s' service's configuration", name, s.Name)
}

// GetFirstController returns the controller without requiring its name.
// If the service doesn't have controllers, then it will return an error.
func (s *Service) GetFirstController() (Controller, error) {
	if len(s.Controllers) == 0 {
		return Controller{}, fmt.Errorf("service '%s' doesn't have any controllers in yaml file", s.Name)
	}

	controller := s.Controllers[0]
	return controller, nil
}

// GetExtension returns the extension configuration by the extension name.
// If the extension doesn't exist, then it returns an error.
func (s *Service) GetExtension(name string) (Extension, error) {
	for _, e := range s.Extensions {
		if e.Name == name {
			return e, nil
		}
	}

	return Extension{}, fmt.Errorf("'%s' extension was not found in '%s' service's configuration", name, s.Name)
}

type Services []Service
