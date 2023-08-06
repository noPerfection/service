package service

import (
	"fmt"
	"github.com/ahmetson/service-lib/os/network"
)

type Instance struct {
	Port               uint64
	Id                 string
	ControllerCategory string
}

func NewInstance(cat string) (*Instance, error) {
	port := network.GetFreePort()
	if port == 0 {
		return nil, fmt.Errorf("network.GetFreePort: no free port")
	}

	sourceInstance := Instance{
		ControllerCategory: cat,
		Id:                 cat + "01",
		Port:               uint64(port),
	}

	return &sourceInstance, nil
}
