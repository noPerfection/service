package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/os/arg"
	"github.com/noPerfection/protocol/client"
	"github.com/noPerfection/protocol/message"
)

const requestCount = 5

func main() {
	port := uint64(8000)
	if value := arg.FlagValue("port"); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			panic(fmt.Errorf("parse --port=%s: %w", value, err))
		}
		port = parsed
	}

	handlerType := client.ReplierType
	if port == 3000 {
		handlerType = client.SyncReplierType
	}

	rand.Seed(time.Now().UnixNano())
	startedAt := time.Now()

	var wg sync.WaitGroup
	errs := make(chan error, requestCount)
	results := make(chan string, requestCount)

	for i := 1; i <= requestCount; i++ {
		i := i
		code := randomCode()
		wg.Add(1)
		go func() {
			defer wg.Done()

			duration, msg, err := callHello(port, handlerType, code)
			if err != nil {
				errs <- fmt.Errorf("request #%d: %w", i, err)
				return
			}
			if i == 1 {
				results <- fmt.Sprintf("request #1 code=%s reply=%q time=%s", code, msg, duration.Round(time.Millisecond))
			}
		}()
	}

	wg.Wait()
	close(errs)
	close(results)

	for err := range errs {
		if err != nil {
			panic(err)
		}
	}
	for result := range results {
		fmt.Println(result)
	}

	fmt.Printf("total time for %d clients: %s\n", requestCount, time.Since(startedAt).Round(time.Millisecond))
}

func callHello(port uint64, handlerType client.HandlerType, code string) (time.Duration, string, error) {
	c, err := client.New("localhost", port, handlerType)
	if err != nil {
		return 0, "", err
	}
	defer c.Close()

	c.Timeout(2 * time.Second)
	c.Attempt(3)

	startedAt := time.Now()
	reply, err := c.Request(&message.Request{
		Command:    "hello",
		Parameters: datatype.New().Set("code", code),
	})
	if err != nil {
		return 0, "", err
	}
	if !reply.IsOK() {
		return 0, "", fmt.Errorf(reply.ErrorMessage())
	}

	msg, err := reply.ReplyParameters().StringValue("message")
	if err != nil {
		return 0, "", err
	}
	return time.Since(startedAt), msg, nil
}

func randomCode() string {
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}
