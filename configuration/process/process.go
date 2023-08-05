package process

import (
	"fmt"
	"github.com/cakturk/go-netstat/netstat"
	"os"
)

func CurrentPid() uint64 {
	return uint64(os.Getpid())
}

func PortToPid[V int | uint64](port V) (uint64, error) {
	socks, err := netstat.TCPSocks(func(s *netstat.SockTabEntry) bool {
		return s.LocalAddr.Port == uint16(port)
	})
	if err != nil {
		return 0, fmt.Errorf("netstart.TCPSocks: %w", err)
	}
	if len(socks) == 0 {
		return 0, fmt.Errorf("no process on port %d: %w", port, err)
	}
	sock := socks[0]

	return uint64(sock.Process.Pid), nil
}
