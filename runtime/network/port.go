package network

import (
	"fmt"
	"github.com/phayes/freeport"
	"net"
	"time"
)

// GetFreePort returns a TCP port to use, if there is no port, it will return 0
func GetFreePort() int {
	port, err := freeport.GetFreePort()
	if err != nil {
		return 0
	}

	return port
}

func IsPortUsed[V int | uint64](host string, port V) bool {
	portString := fmt.Sprintf("%d", port)
	timeout := time.Second
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, portString), timeout)
	if err != nil {
		return false
	}
	if conn != nil {
		err := conn.Close()
		if err != nil {
			return false
		}
	}
	return true
}
