package main

import (
	"github.com/noPerfection/service/examples/011-security/services/proxy2"
)

func main() {
	app, err := proxy2.New()
	if err != nil {
		panic(err)
	}

	if err := app.Start(); err != nil {
		panic(err)
	}
	defer app.Stop()

	app.Wait()
}
