package service

type Controller struct {
	Type      ControllerType
	Category  string
	Instances []Instance
}

func NewController(as ControllerType, cat string) *Controller {
	control := &Controller{
		Type:     as,
		Category: cat,
	}

	return control
}
