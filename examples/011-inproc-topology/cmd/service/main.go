package main

import "github.com/noPerfection/service/examples/011-inproc-topology/goroutines"

func main() {
	done := make(chan error, 1)
	go goroutines.Start(done)

	if err := <-done; err != nil {
		panic(err)
	}
}
