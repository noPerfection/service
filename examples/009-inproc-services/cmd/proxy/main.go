package main

import (
	"fmt"

	"github.com/noPerfection/service/examples/009-inproc-services/services/proxy"
)

func main() {
	app, err := proxy.New()
	if err != nil {
		panic(err)
	}

	if err := app.Start(); err != nil {
		panic(err)
	}
	defer app.Stop()

	fmt.Println("default-name-proxy listening on tmp/default_name_proxy")

	app.Wait()
}