package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"svn-code-reviewer/internal/config"
)

type OpenAIClient struct {
	apiKey      string
	baseURL     string
	model       string
	temperature float32
	maxTokens   int
	httpClient  *http.Client
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float32       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type chatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func NewOpenAIClient(cfg *config.AIConfig) *OpenAIClient {
	return &OpenAIClient{
		apiKey:      cfg.APIKey,
		baseURL:     cfg.BaseURL,
		model:       cfg.Model,
		temperature: cfg.Temperature,
		maxTokens:   cfg.MaxTokens,
		httpClient:  &http.Client{},
	}
}

func (c *OpenAIClient) Review(ctx context.Context, fileName, diff, systemPrompt string) (*ReviewResult, error) {
	userPrompt := fmt.Sprintf("文件名: %s\n\n代码变更:\n```\n%s\n```\n\n请审核以上代码变更。", fileName, diff)

	reqBody := chatRequest{
		Model:       c.model,
		Temperature: c.temperature,
		MaxTokens:   c.maxTokens,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return &ReviewResult{
			FileName: fileName,
			Success:  false,
			Error:    fmt.Errorf("序列化请求失败: %w", err),
		}, fmt.Errorf("序列化请求失败: %w", err)
	}

	url := c.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return &ReviewResult{
			FileName: fileName,
			Success:  false,
			Error:    fmt.Errorf("创建请求失败: %w", err),
		}, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &ReviewResult{
			FileName: fileName,
			Success:  false,
			Error:    fmt.Errorf("API 请求失败: %w", err),
		}, fmt.Errorf("API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ReviewResult{
			FileName: fileName,
			Success:  false,
			Error:    fmt.Errorf("读取响应失败: %w", err),
		}, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &ReviewResult{
			FileName: fileName,
			Success:  false,
			Error:    fmt.Errorf("API 返回错误状态码: %d\n响应内容:\n%s", resp.StatusCode, string(body)),
		}, fmt.Errorf("API 返回错误状态码: %d\n响应内容:\n%s", resp.StatusCode, string(body))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return &ReviewResult{
			FileName: fileName,
			Success:  false,
			Error:    fmt.Errorf("解析响应失败: %w\n原始响应:\n%s", err, string(body)),
		}, fmt.Errorf("解析响应失败: %w\n原始响应:\n%s", err, string(body))
	}

	// 输出完整响应用于调试
	if len(chatResp.Choices) == 0 {
		respJSON, _ := json.MarshalIndent(chatResp, "", "  ")
		errMsg := fmt.Sprintf("AI 返回空响应\n完整响应对象:\n%s", string(respJSON))
		return &ReviewResult{
			FileName: fileName,
			Success:  false,
			Error:    fmt.Errorf(errMsg),
		}, fmt.Errorf(errMsg)
	}

	return &ReviewResult{
		FileName: fileName,
		Content:  chatResp.Choices[0].Message.Content,
		Success:  true,
	}, nil
}
