package goroutines

import "github.com/noPerfection/service/inproc_topology"

func Start(done chan<- error) {
	topology := &inproc_topology.InprocTopology{}
	done <- topology.Start()
}
