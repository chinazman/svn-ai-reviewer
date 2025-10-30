package ai

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
	"svn-code-reviewer/internal/config"
)

type OpenAIClient struct {
	client      *openai.Client
	model       string
	temperature float32
	maxTokens   int
}

func NewOpenAIClient(cfg *config.AIConfig) *OpenAIClient {
	clientConfig := openai.DefaultConfig(cfg.APIKey)
	
	// 如果配置了自定义 BaseURL，则使用它
	if cfg.BaseURL != "" {
		clientConfig.BaseURL = cfg.BaseURL
	}

	return &OpenAIClient{
		client:      openai.NewClientWithConfig(clientConfig),
		model:       cfg.Model,
		temperature: cfg.Temperature,
		maxTokens:   cfg.MaxTokens,
	}
}

func (c *OpenAIClient) Review(ctx context.Context, fileName, diff, systemPrompt string) (*ReviewResult, error) {
	userPrompt := fmt.Sprintf("文件名: %s\n\n代码变更:\n```\n%s\n```\n\n请审核以上代码变更。", fileName, diff)

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       c.model,
		Temperature: c.temperature,
		MaxTokens:   c.maxTokens,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		},
	})

	if err != nil {
		return &ReviewResult{
			FileName: fileName,
			Success:  false,
			Error:    err,
		}, err
	}

	if len(resp.Choices) == 0 {
		return &ReviewResult{
			FileName: fileName,
			Success:  false,
			Error:    fmt.Errorf("AI 返回空响应"),
		}, fmt.Errorf("AI 返回空响应")
	}

	return &ReviewResult{
		FileName: fileName,
		Content:  resp.Choices[0].Message.Content,
		Success:  true,
	}, nil
}
