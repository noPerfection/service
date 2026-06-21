package main

import (
	"fmt"
	"sync"

	"github.com/noPerfection/service/examples/009-single-process/entrypoint"
	"github.com/noPerfection/service/examples/009-single-process/hello"
	"github.com/noPerfection/service/examples/009-single-process/proxy"
)

const configPath = "noPerfection.json"

type waitable interface {
	Wait()
}

func main() {
	defaultProxy, err := proxy.New(configPath)
	if err != nil {
		panic(err)
	}
	if err := defaultProxy.Start(); err != nil {
		panic(err)
	}
	defer defaultProxy.Stop()

	entrypointProxy, err := entrypoint.New(configPath)
	if err != nil {
		panic(err)
	}
	if err := entrypointProxy.Start(); err != nil {
		panic(err)
	}
	defer entrypointProxy.Stop()

	helloService, err := hello.New(configPath)
	if err != nil {
		panic(err)
	}
	if err := helloService.Start(); err != nil {
		panic(err)
	}
	defer helloService.Stop()

	fmt.Println("single-process demo started")
	fmt.Println("hello-world service listening on localhost:8000")
	fmt.Println("default-name-proxy listening on localhost:8002")
	fmt.Println("entrypoint proxy listening on localhost:8004")

	waitAll(defaultProxy, entrypointProxy, helloService)
}

func waitAll(services ...waitable) {
	var wg sync.WaitGroup
	wg.Add(len(services))
	for _, service := range services {
		go func(service waitable) {
			defer wg.Done()
			service.Wait()
		}(service)
	}
	wg.Wait()
}
