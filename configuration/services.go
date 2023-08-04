package configuration

import (
	"fmt"
	"strings"
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
	Url       string
	Instances []ControllerInstance
	Context   ContextType
}

type Extension struct {
	Url      string
	Instance string
	Port     uint64
}

type PipeEnd struct {
	Name string
	Url  string
}

type Pipeline struct {
	End  *PipeEnd
	Head []string
}

// Service type defined in the configuration
type Service struct {
	Type        ServiceType
	Url         string
	Instance    string
	Controllers []Controller
	Proxies     []Proxy
	Extensions  []Extension
	Pipelines   []Pipeline
}

// NewControllerPipeEnd creates a pipe end with the given name as the controller
func NewControllerPipeEnd(end string) *PipeEnd {
	return &PipeEnd{
		Url:  "",
		Name: end,
	}
}

func NewThisServicePipeEnd() *PipeEnd {
	return NewControllerPipeEnd("")
}

func (end *PipeEnd) IsController() bool {
	return len(end.Name) > 0
}

func (end *PipeEnd) Pipeline(head []string) *Pipeline {
	return &Pipeline{
		End:  end,
		Head: head,
	}
}

func HasServicePipeline(pipelines []Pipeline) bool {
	for _, pipeline := range pipelines {
		if !pipeline.End.IsController() {
			return true
		}
	}

	return false
}

func ControllerPipelines(allPipelines []Pipeline) []Pipeline {
	pipelines := make([]Pipeline, 0, len(allPipelines))
	count := 0

	for _, pipeline := range allPipelines {
		if pipeline.End.IsController() {
			pipelines[count] = pipeline
			count++
		}
	}

	return pipelines
}

func ServicePipeline(allPipelines []Pipeline) *Pipeline {
	for _, pipeline := range allPipelines {
		if !pipeline.End.IsController() {
			return &pipeline
		}
	}

	return nil
}

// ValidateHead makes sure that pipeline has proxies, and they are all proxies are unique
func (pipeline *Pipeline) ValidateHead() error {
	if !pipeline.HasBeginning() {
		return fmt.Errorf("no head")
	}
	last := len(pipeline.Head) - 1
	for i := 0; i < last; i++ {
		needle := pipeline.Head[i]

		for j := i + 1; j <= last; j++ {
			url := pipeline.Head[j]
			if strings.Compare(url, needle) == 0 {
				return fmt.Errorf("the %d and %d proxies in the head are duplicates", i, j)
			}
		}
	}

	return nil
}

func (pipeline *Pipeline) HasLength() bool {
	return pipeline.End != nil && pipeline.HasBeginning()
}

func (pipeline *Pipeline) IsMultiHead() bool {
	return len(pipeline.Head) > 1
}

// HeadFront returns all proxy except the last one.
func (pipeline *Pipeline) HeadFront() []string {
	return pipeline.Head[:len(pipeline.Head)-1]
}

// HeadLast returns the last proxy in the proxy chain
func (pipeline *Pipeline) HeadLast() string {
	return pipeline.Head[len(pipeline.Head)-1]
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
	return &Extension{Url: name, Port: 0}
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

// Beginning Returns the first proxy url in the pipeline. Doesn't validate it.
// Call first HasBeginning to check does the pipeline have a beginning
func (pipeline *Pipeline) Beginning() string {
	return pipeline.Head[0]
}

func (pipeline *Pipeline) HasBeginning() bool {
	return len(pipeline.Head) > 0
}

// GetController returns the controller configuration by the controller name.
// If the controller doesn't exist, then it returns an error.
func (s *Service) GetController(name string) (Controller, error) {
	for _, c := range s.Controllers {
		if c.Name == name {
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
		if c.Name == name {
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
func ServiceToProxy(s *Service, contextType ContextType) (Proxy, error) {
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

	instance := ControllerInstance{
		Instance: controllerConfig.Name + " instance 01",
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
		Instances: []ControllerInstance{instance},
		Context:   contextType,
	}

	return converted, nil
}

// findPipelineBeginning returns the beginning of the pipeline.
// If the contextType is not a default one, then it will search for the specific context type.
func findPipelineBeginning(s *Service, requiredEnd string, contextType ContextType) (*Proxy, error) {
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

		if contextType != DefaultContext {
			if proxy.Context != contextType {
				continue
			} else {
				return proxy, nil
			}
		} else {
			if proxy.Context == DefaultContext {
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
func (s *Service) HasProxy(contextType ContextType) bool {
	if len(s.Proxies) == 0 {
		return false
	}
	if contextType == DefaultContext {
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
func ServiceToExtension(s *Service, contextType ContextType) (Extension, error) {
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
		Url:      s.Url,
		Instance: controllerConfig.Name + " instance 01",
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
