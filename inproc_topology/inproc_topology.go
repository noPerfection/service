package inproc_topology

import (
	"fmt"

	"github.com/noPerfection/service/package_url"
)

type InprocTopology struct {
}

func (topology *InprocTopology) Start() error {
	pkgInfo, err := package_url.GetPackageInfo()
	if err != nil {
		return err
	}

	fmt.Println(pkgInfo)
	return nil
}
