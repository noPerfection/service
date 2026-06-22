package inproc_topology

import (
	"github.com/noPerfection/service/package_url"
)

type InprocTopology struct {
}

func (topology *InprocTopology) Start() error {
	pkgInfo, err := package_url.GetPackageInfo()
	if err != nil {
		return err
	}

	pkgInfo.Print()
	return nil
}
