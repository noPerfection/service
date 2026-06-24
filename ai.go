package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/mozilla-ai/any-llm-go/providers/anthropic"
	"github.com/noPerfection/protocol/message"
)

const (
	AiServiceName  = "ai"
	defaultAiModel = "claude-haiku-4-5-20251001"
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
		ModuleUrl: "pkg:golang/github.com/noPerfection/service#service?func=NewAiService()",
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
	}
}

// AiService is the noPerfection AI extension.
type AiService struct {
	*Extension
	apiKey   string
	model    string
	provider anyllm.Provider
	running  bool
}

// NewAiService returns an AI extension service.
//
// Optional arguments, in order:
//  1. apiKey — supply from the caller after loading env (e.g. os.Getenv after env.LoadAnyEnv).
//  2. model — when omitted, defaultAiModel is used.
//
// Check out the model in mozilla-ai/any-llm-go/providers/anthropic
func NewAiService(apiKeyAndModel ...string) (*AiService, error) {
	if len(apiKeyAndModel) > 2 {
		return nil, fmt.Errorf("too many arguments, expected api key and model")
	}

	apiKey := ""
	if len(apiKeyAndModel) > 0 {
		apiKey = apiKeyAndModel[0]
	}

	selectedModel := defaultAiModel
	if len(apiKeyAndModel) > 1 && apiKeyAndModel[1] != "" {
		selectedModel = apiKeyAndModel[1]
	}

	extension, err := NewExt(AiServiceName, DefaultConfigPath, DefaultAiManagerEndpoint)
	if err != nil {
		return nil, err
	}

	err = extension.SetServiceConfig(defaultAiExtensionServiceConfig())
	if err != nil {
		return nil, err
	}

	ai := &AiService{
		Extension: extension,
		apiKey:    apiKey,
		model:     selectedModel,
		running:   false,
	}

	if apiKey != "" {
		provider, err := ai.anthropicProvider()
		if err != nil {
			return nil, fmt.Errorf("ai failed: %w", err)
		}
		ai.provider = provider
	}

	return ai, nil
}

func (ai *AiService) ensureProvider() error {
	if ai == nil {
		return fmt.Errorf("ai service is nil")
	}
	if ai.provider != nil {
		return nil
	}
	if ai.apiKey == "" {
		return fmt.Errorf("api key is empty")
	}
	provider, err := ai.anthropicProvider()
	if err != nil {
		return fmt.Errorf("ai failed: %w", err)
	}
	ai.provider = provider
	return nil
}

func (ai *AiService) anthropicProvider() (anyllm.Provider, error) {
	if ai == nil {
		return nil, fmt.Errorf("ai service is nil")
	}
	if ai.apiKey == "" {
		return nil, fmt.Errorf("api key is empty")
	}
	return anthropic.New(anyllm.WithAPIKey(ai.apiKey))
}

func (ai *AiService) Start() error {
	if ai == nil {
		return fmt.Errorf("ai service is nil")
	}
	if ai.running {
		return fmt.Errorf("ai service is already running")
	}

	err := ai.CheckConnection()
	if err != nil {
		return err
	}

	ai.running = true
	return ai.Extension.Start()
}

// CheckConnection verifies that the Anthropic API key can make a minimal completion.
func (ai *AiService) CheckConnection() error {
	if err := ai.ensureProvider(); err != nil {
		return err
	}

	checkContent := "ara.foundation"
	maxTokens := 1

	if ai.logger != nil {
		ai.logger.Info(fmt.Sprintf("Maydan --> %s: %s", ai.model, checkContent))
	}
	reply, err := ai.provider.Completion(context.Background(), anyllm.CompletionParams{
		Model: ai.model,
		Messages: []anyllm.Message{
			{Role: anyllm.RoleUser, Content: checkContent},
		},
		MaxTokens: &maxTokens,
	})
	if err != nil {
		errorStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("202"))
		if ai.logger != nil {
			ai.logger.Error(fmt.Sprintf("Maydan -/- %s: %s", ai.model, errorStyle.Render("connection failed")))
		}
		return err
	}

	replyContent := ""
	if len(reply.Choices) > 0 {
		replyContent = fmt.Sprint(reply.Choices[0].Message.Content)
	}
	if ai.logger != nil {
		ai.logger.Info(fmt.Sprintf("Maydan <-- %s: '%s'", ai.model, replyContent))
	}

	return nil
}

func (ai *AiService) GenerateComposeServiceBlock(repoName, serviceName, buildContext, dockerfile string) (string, error) {
	if err := ai.ensureProvider(); err != nil {
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
		ai.logger.Info(fmt.Sprintf("Maydan --> %s: compose service for %s", ai.model, repoName))
	}
	reply, err := ai.provider.Completion(context.Background(), anyllm.CompletionParams{
		Model: ai.model,
		Messages: []anyllm.Message{
			{Role: anyllm.RoleUser, Content: prompt},
		},
		MaxTokens:   &maxTokens,
		Temperature: &temperature,
	})
	if err != nil {
		errorStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("202"))
		if ai.logger != nil {
			ai.logger.Error(fmt.Sprintf("Maydan -/- %s: %s", ai.model, errorStyle.Render("connection failed")))
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
		ai.logger.Info(fmt.Sprintf("Maydan <-- %s: service %s", ai.model, serviceName))
	}
	return content, nil
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
