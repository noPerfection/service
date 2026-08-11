package service

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/noPerfection/service/zap"
)

func waitForShutdown(blocker *sync.WaitGroup, stop func() error) {
	if blocker == nil {
		return
	}

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	<-sigCh
	_ = stop()
	zap.Stop()

	done := make(chan struct{})
	go func() {
		blocker.Wait()
		close(done)
	}()

	select {
	case <-done:
		return
	case <-sigCh:
		os.Exit(130)
	case <-time.After(3 * time.Second):
		os.Exit(130)
	}
}

func releaseBlocker(blocker *sync.WaitGroup) {
	if blocker == nil {
		return
	}
	go func() {
		done := make(chan struct{})
		go func() {
			blocker.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(time.Second):
			// WaitGroup still blocked; waitForShutdown fallback will os.Exit.
		}
	}()
}
