package service

import (
	"fmt"
	"github.com/ahmetson/os-lib/net"
)

type Instance struct {
	Port               uint64
	Id                 string
	ControllerCategory string
}

func NewInstance(cat string) (*Instance, error) {
	port := net.GetFreePort()
	if port == 0 {
		return nil, fmt.Errorf("net.GetFreePort: no free port")
	}

	sourceInstance := Instance{
		ControllerCategory: cat,
		Id:                 cat + "01",
		Port:               uint64(port),
	}

	return &sourceInstance, nil
}
