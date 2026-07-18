package service

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func waitForShutdown(blocker *sync.WaitGroup, stop func() error) {
	if blocker == nil {
		return
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	go func() {
		<-sigCh
		_ = stop()
	}()

	blocker.Wait()
}
