package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/mozilla-ai/any-llm-go/providers/anthropic"
	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/message"
)

const (
	AiServiceName               = "ai"
	MainPackageToLibraryCommand = "main-package-to-library"
	defaultAiModel              = "claude-haiku-4-5-20251001"
	aiAPIKeyParameter           = "api-key"
	aiModelParameter            = "model"
	defaultAiAPIKeyLink         = "*pkg:os/env?var=ANTHROPIC_API_KEY&LoadAnyEnv=true"
)

var (
	DefaultAiEndpoint        = message.NewEndpoint(AiServiceName, 0)
	DefaultAiManagerEndpoint = message.NewEndpoint(AiServiceName+"_manager", 0)
)

func aiExtensionServiceLink() string {
	return "pkg:$?var=services[name:" + AiServiceName + "]"
}

func defaultAiExtensionServiceConfig() Config {
	return Config{
		Type:      ExtensionType,
		Name:      AiServiceName,
		ModuleUrl: "pkg:golang/github.com/noPerfection/service",
		Handlers: []Handler{
			IndependentHandler{
				Type:     SyncReplierType,
				Category: ServiceManagerCategory,
				Endpoint: DefaultAiManagerEndpoint,
			},
			ExtensionHandler{
				IndependentHandler: IndependentHandler{
					Type:     ReplierType,
					Category: "main",
					Endpoint: DefaultAiEndpoint,
				},
			},
		},
		Parameters: datatype.New().
			Set(aiModelParameter, defaultAiModel).
			Set(aiAPIKeyParameter, defaultAiAPIKeyLink),
	}
}

func aiParametersFromConfig(serviceConfig Config) (apiKey string, model string, err error) {
	model = defaultAiModel
	if serviceConfig.Parameters == nil {
		return "", model, nil
	}
	if value, err := serviceConfig.Parameters.StringValue(aiModelParameter); err == nil && value != "" {
		model = value
	}
	if value, err := serviceConfig.Parameters.StringValue(aiAPIKeyParameter); err == nil {
		apiKey = value
	}
	return apiKey, model, nil
}

// AiService is the noPerfection AI extension.
type AiService struct {
	*Extension
	provider anyllm.Provider
	running  bool
}

// NewAiService returns an AI extension service.
//
// configPath is the topology JSON path (for example noPerfection.json). When omitted,
// DefaultConfigPath is used. The ai service record is registered in topology when
// missing; parameters are read from topology whenever the provider is used.
//
// To set api-key or model before Start:
//
//		ai, err := NewAiService()
//		err = ai.SetServiceParams(
//	   		service.KeyValue().
//				Set("api-key", "your key").
//				Set("model", "model-name"),
//	   	)
//		if err != nil { ... }
//
// Api key will be then stored in the topology configuration, if you don't want to then pass the mushroom dereference.
//
// Default parameters:
//   - api-key — "*pkg:os/env?var=ANTHROPIC_API_KEY&LoadAnyEnv=true"
//   - model — "claude-haiku-4-5-20251001"
//
// Models: mozilla-ai/any-llm-go/providers/anthropic
func NewAiService(configPath ...string) (*AiService, error) {
	path := DefaultConfigPath
	if len(configPath) > 0 {
		path = configPath[0]
	}

	extension, err := NewExt(AiServiceName, path, DefaultAiManagerEndpoint)
	if err != nil {
		return nil, err
	}

	ai := &AiService{Extension: extension}
	if err := ai.registerInTopology(); err != nil {
		return nil, err
	}
	if err := ai.registerRoutes(); err != nil {
		return nil, err
	}
	return ai, nil
}

func (ai *AiService) registerRoutes() error {
	return ai.Route(MainPackageToLibraryCommand, func(req RequestInterface) ReplyInterface {
		packageName, err := req.RouteParameters().StringValue("package-name")
		if err != nil || packageName == "" {
			return req.Fail("package-name is required")
		}
		importClause, err := req.RouteParameters().StringValue("import-clause")
		if err != nil || importClause == "" {
			return req.Fail("import-clause is required")
		}
		mainGo, err := req.RouteParameters().StringValue("main-go")
		if err != nil || mainGo == "" {
			return req.Fail("main-go is required")
		}
		serviceCode, updatedMain, err := ai.mainPackageToLibrary(packageName, importClause, mainGo)
		if err != nil {
			return req.Fail(err.Error())
		}
		return req.Ok(KeyValue().
			Set("service-code", serviceCode).
			Set("main-go", updatedMain))
	})
}

func (ai *AiService) registerInTopology() error {
	if ai == nil || ai.topologyHandler == nil {
		return fmt.Errorf("ai service or topology handler is nil")
	}
	_, err := ai.topologyHandler.Service(AiServiceName)
	if err == nil {
		return nil
	}
	if err := ai.topologyHandler.AddService(defaultAiExtensionServiceConfig()); err != nil {
		return fmt.Errorf("topologyHandler.AddService(%q): %w", AiServiceName, err)
	}
	return nil
}

func (ai *AiService) ensureProvider() (model string, err error) {
	if ai == nil {
		return "", fmt.Errorf("ai service is nil")
	}

	serviceConfig, err := ai.topology().Service(ai.mushroomURL)
	if err != nil {
		return "", err
	}
	apiKey, model, err := aiParametersFromConfig(serviceConfig)
	if err != nil {
		return "", err
	}
	if apiKey == "" {
		return model, fmt.Errorf("api key is empty")
	}

	ai.provider, err = anthropic.New(anyllm.WithAPIKey(apiKey))
	if err != nil {
		return model, fmt.Errorf("ai failed: %w", err)
	}
	return model, nil
}

func (ai *AiService) Start() error {
	if ai == nil {
		return fmt.Errorf("ai service is nil")
	}
	if ai.running {
		return fmt.Errorf("ai service is already running")
	}

	ai.running = true
	return ai.Extension.Start()
}

// CheckConnection verifies that the Anthropic API key can make a minimal completion.
func (ai *AiService) CheckConnection() error {
	model, err := ai.ensureProvider()
	if err != nil {
		return err
	}

	checkContent := "ara.foundation"
	maxTokens := 1

	if ai.logger != nil {
		ai.logger.Info(fmt.Sprintf("Maydan --> %s: %s", model, checkContent))
	}
	reply, err := ai.provider.Completion(context.Background(), anyllm.CompletionParams{
		Model: model,
		Messages: []anyllm.Message{
			{Role: anyllm.RoleUser, Content: checkContent},
		},
		MaxTokens: &maxTokens,
	})
	if err != nil {
		errorStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("202"))
		if ai.logger != nil {
			ai.logger.Error(fmt.Sprintf("Maydan -/- %s: %s", model, errorStyle.Render("connection failed")))
		}
		return err
	}

	replyContent := ""
	if len(reply.Choices) > 0 {
		replyContent = fmt.Sprint(reply.Choices[0].Message.Content)
	}
	if ai.logger != nil {
		ai.logger.Info(fmt.Sprintf("Maydan <-- %s: '%s'", model, replyContent))
	}

	return nil
}

func (ai *AiService) GenerateComposeServiceBlock(repoName, serviceName, buildContext, dockerfile string) (string, error) {
	model, err := ai.ensureProvider()
	if err != nil {
		return "", err
	}

	maxTokens := 1200
	temperature := 0.0

	prompt := fmt.Sprintf(`Generate a Docker Compose YAML document for this repository.

Rules:
- Return only YAML. Do not use markdown fences or commentary.
- Include a top-level "services:" key.
- Include exactly one service named %q.
- The service must build from context %q and Dockerfile "Dockerfile".
- Add a volumes mount binding the build context directory to the container's WORKDIR: "<build-context>:<workdir-in-container>" using use the last WORKDIR in the Dockerfile”.
- Do NOT include working_dir, entrypoint, or cmd if they are already defined in the Dockerfile.
- Do NOT include environment variables, secrets, credentials, or host-specific paths unless clearly exposed in the Dockerfile via ENV or EXPOSE.
- Prefer the smallest valid service definition.
- Only include ports if the Dockerfile has an EXPOSE instruction.

Repository: %s

Dockerfile:
%s
`, serviceName, buildContext, repoName, dockerfile)

	if ai.logger != nil {
		ai.logger.Info(fmt.Sprintf("Maydan --> %s: compose service for %s", model, repoName))
	}
	reply, err := ai.provider.Completion(context.Background(), anyllm.CompletionParams{
		Model: model,
		Messages: []anyllm.Message{
			{Role: anyllm.RoleUser, Content: prompt},
		},
		MaxTokens:   &maxTokens,
		Temperature: &temperature,
	})
	if err != nil {
		errorStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("202"))
		if ai.logger != nil {
			ai.logger.Error(fmt.Sprintf("Maydan -/- %s: %s", model, errorStyle.Render("connection failed")))
		}
		return "", err
	}
	if len(reply.Choices) == 0 {
		return "", fmt.Errorf("Claude returned no choices")
	}

	content := strings.TrimSpace(messageContent(reply.Choices[0].Message.Content))
	if content == "" {
		return "", fmt.Errorf("Claude returned an empty compose block")
	}
	if ai.logger != nil {
		ai.logger.Info(fmt.Sprintf("Maydan <-- %s: service %s", model, serviceName))
	}
	return content, nil
}

// MainPackageToLibrary converts a main package into a reusable service library plus a slim main.go.
// When the ai service is running, call via AiClient only; direct use is rejected.
func (ai *AiService) MainPackageToLibrary(packageName, importClause, mainGo string) (serviceCode string, updatedMain string, err error) {
	if ai != nil && ai.running {
		return "", "", fmt.Errorf("ai service is running: call MainPackageToLibrary via AiClient only")
	}
	return ai.mainPackageToLibrary(packageName, importClause, mainGo)
}

func (ai *AiService) mainPackageToLibrary(packageName, importClause, mainGo string) (serviceCode string, updatedMain string, err error) {
	model, err := ai.ensureProvider()
	if err != nil {
		return "", "", err
	}

	maxTokens := 8192
	temperature := 0.0

	prompt := fmt.Sprintf(`Given the following main package, extract the service initialization into a separate library.

Rules:
1. Find the service constructor call — one of:
   - github.com/noPerfection/service.New()
   - github.com/noPerfection/service.NewProxy()
   - github.com/noPerfection/service.NewExt()

2. Extract service code:
   - A New() function that contains all setup code (constructor, routes, configs)
   - New() returns the service type (*service.Independent | *service.Proxy | *service.Extension) and error
   - All handler functions and constants used by the service
   - Runtime-only code (fmt.Println, os signals) stays in main.go

3. Update main.go to:
   - Import the new package using: import %s
   - Call %s.New() and handle the error
   - Call app.Start() and app.Wait() directly
   - Keep defer app.Stop()

Return two code blocks only:

1. Service code (handlers, New() function, constants)
2. Updated main.go

Input:
Package name: %s
Import clause: %s
Main.go:
%s
`, importClause, packageName, packageName, importClause, mainGo)

	if ai.logger != nil {
		ai.logger.Info(fmt.Sprintf("Maydan --> %s: main package to library %s", model, packageName))
	}
	reply, err := ai.provider.Completion(context.Background(), anyllm.CompletionParams{
		Model: model,
		Messages: []anyllm.Message{
			{Role: anyllm.RoleUser, Content: prompt},
		},
		MaxTokens:   &maxTokens,
		Temperature: &temperature,
	})
	if err != nil {
		errorStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("202"))
		if ai.logger != nil {
			ai.logger.Error(fmt.Sprintf("Maydan -/- %s: %s", model, errorStyle.Render("connection failed")))
		}
		return "", "", err
	}
	if len(reply.Choices) == 0 {
		return "", "", fmt.Errorf("Claude returned no choices")
	}

	content := strings.TrimSpace(messageContent(reply.Choices[0].Message.Content))
	serviceCode, updatedMain, err = parseTwoCodeBlocks(content)
	if err != nil {
		return "", "", err
	}
	if ai.logger != nil {
		ai.logger.Info(fmt.Sprintf("Maydan <-- %s: library %s", model, packageName))
	}
	return serviceCode, updatedMain, nil
}

func parseTwoCodeBlocks(content string) (string, string, error) {
	var blocks []string
	rest := content
	for len(blocks) < 2 {
		start := strings.Index(rest, "```")
		if start == -1 {
			break
		}
		rest = rest[start+3:]
		if nl := strings.Index(rest, "\n"); nl != -1 {
			rest = rest[nl+1:]
		}
		end := strings.Index(rest, "```")
		if end == -1 {
			return "", "", fmt.Errorf("unclosed code block in model response")
		}
		blocks = append(blocks, strings.TrimSpace(rest[:end]))
		rest = rest[end+3:]
	}
	if len(blocks) < 2 {
		return "", "", fmt.Errorf("expected two code blocks, got %d", len(blocks))
	}
	return blocks[0], blocks[1], nil
}

func messageContent(content any) string {
	switch c := content.(type) {
	case string:
		return c
	case []anyllm.ContentPart:
		parts := make([]string, 0, len(c))
		for _, part := range c {
			if part.Text != "" {
				parts = append(parts, part.Text)
			}
		}
		return strings.Join(parts, "")
	case []any:
		parts := make([]string, 0, len(c))
		for _, item := range c {
			if part, ok := item.(map[string]any); ok {
				if text, ok := part["text"].(string); ok {
					parts = append(parts, text)
					continue
				}
			}
			parts = append(parts, fmt.Sprint(item))
		}
		return strings.Join(parts, "")
	default:
		return fmt.Sprint(c)
	}
}
