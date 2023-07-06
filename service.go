// Package service is the umbrella for various packages to create SDS Service.
//
// The SDS Services created by this module are
// connected through sockets over TCP.
package service

/*
Example 1. Create an independent service

import (
 "independent"
)

let service = independent.New("name of the independent service", "group")
// It will load the configuration.
// Then it will try to get the exposed proxies.
// Then it will load the extensions.

// define the commands (depending extensions)
// define the controllers
// set the commands to the controllers

service.addController(controllers)

service.Run()

Any service creates the Router. Router can get the commands that user defines
Also it adds the GetServiceName. GetCommands. GetServiceType commands.
*/
