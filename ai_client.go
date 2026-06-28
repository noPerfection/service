package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/client"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/service/handlers"
)

const (
	aiClientTimeout = 2 * time.Minute
)

// AiClient connects to a running ai extension using its topology service configuration.
type AiClient struct {
	mu     sync.Mutex
	client *client.Socket
}

// NewAiClient returns a client connected to the ai extension described by service configuration.
func NewAiClient(aiExtension Config) (*AiClient, error) {
	handler, err := aiExtension.HandlerByCategory(handlers.DefaultHandlerCategory)
	if err != nil {
		return nil, fmt.Errorf("ai handler: %w", err)
	}
	indHandler, ok := handler.AsIndependentHandler()
	if !ok {
		return nil, fmt.Errorf("ai handler is not an independent handler")
	}

	socket, err := Client(indHandler.Endpoint.Id, indHandler.Endpoint.Port)
	if err != nil {
		return nil, fmt.Errorf("ai client: %w", err)
	}
	socket.Timeout(aiClientTimeout).Attempt(1)

	return &AiClient{client: socket}, nil
}

// Close closes the underlying connection.
func (c *AiClient) Close() error {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client == nil {
		return nil
	}
	return c.client.Close()
}

// MainPackageToLibrary asks the ai extension to extract a service library from main.go source.
func (c *AiClient) MainPackageToLibrary(packageName, importClause, mainGo string) (serviceCode string, updatedMain string, err error) {
	if c == nil {
		return "", "", fmt.Errorf("ai client is not connected")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client == nil {
		return "", "", fmt.Errorf("ai client is not connected")
	}

	c.client.Timeout(aiClientTimeout).Attempt(1)
	reply, err := c.client.Request(&message.Request{
		Command: MainPackageToLibraryCommand,
		Parameters: datatype.New().
			Set("package-name", packageName).
			Set("import-clause", importClause).
			Set("main-go", mainGo),
	})
	if err != nil {
		return "", "", fmt.Errorf("request: %w", err)
	}
	if !reply.IsOK() {
		return "", "", fmt.Errorf("%s", reply.ErrorMessage())
	}

	serviceCode, err = reply.ReplyParameters().StringValue("service-code")
	if err != nil {
		return "", "", fmt.Errorf("reply service-code: %w", err)
	}
	updatedMain, err = reply.ReplyParameters().StringValue("main-go")
	if err != nil {
		return "", "", fmt.Errorf("reply main-go: %w", err)
	}
	return serviceCode, updatedMain, nil
}
