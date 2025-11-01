package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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

// makeRequest 发起 API 请求的辅助函数
func (c *OpenAIClient) makeRequest(ctx context.Context, jsonData []byte) (string, error) {
	url := c.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API 请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API 返回错误状态码: %d\n响应内容:\n%s", resp.StatusCode, string(body))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w\n原始响应:\n%s", err, string(body))
	}

	if len(chatResp.Choices) == 0 {
		respJSON, _ := json.MarshalIndent(chatResp, "", "  ")
		return "", fmt.Errorf("AI 返回空响应\n完整响应对象:\n%s", string(respJSON))
	}

	return chatResp.Choices[0].Message.Content, nil
}

func (c *OpenAIClient) Review(ctx context.Context, fileName, diff, systemPrompt string) (*ReviewResult, error) {
	// 检查文件内容大小，避免请求过大
	const maxDiffSize = 50000 // 50KB 限制
	if len(diff) > maxDiffSize {
		truncated := diff[:maxDiffSize]
		diff = truncated + fmt.Sprintf("\n\n... (内容过长，已截断，原始大小: %d 字节)", len(diff))
	}
	
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

	// 第一次请求
	content, err := c.makeRequest(ctx, jsonData)
	if err != nil {
		return &ReviewResult{
			FileName: fileName,
			Success:  false,
			Error:    err,
		}, err
	}

	// 尝试解析 JSON
	var reviewData ReviewJSON
	cleanContent := strings.TrimSpace(content)
	cleanContent = strings.TrimPrefix(cleanContent, "```json")
	cleanContent = strings.TrimPrefix(cleanContent, "```")
	cleanContent = strings.TrimSuffix(cleanContent, "```")
	cleanContent = strings.TrimSpace(cleanContent)

	parseErr := json.Unmarshal([]byte(cleanContent), &reviewData)
	
	// 如果 JSON 解析失败，重试一次
	if parseErr != nil {
		fmt.Printf("  [警告] JSON 解析失败: %v\n", parseErr)
		fmt.Printf("  [警告] 原始内容: %s\n", cleanContent[:min(200, len(cleanContent))])
		fmt.Printf("  [信息] 正在重试请求...\n")
		
		retryContent, retryErr := c.makeRequest(ctx, jsonData)
		if retryErr != nil {
			fmt.Printf("  [警告] 重试请求失败: %v，使用原始响应\n", retryErr)
		} else {
			retryCleanContent := strings.TrimSpace(retryContent)
			retryCleanContent = strings.TrimPrefix(retryCleanContent, "```json")
			retryCleanContent = strings.TrimPrefix(retryCleanContent, "```")
			retryCleanContent = strings.TrimSuffix(retryCleanContent, "```")
			retryCleanContent = strings.TrimSpace(retryCleanContent)
			
			if retryParseErr := json.Unmarshal([]byte(retryCleanContent), &reviewData); retryParseErr == nil {
				fmt.Printf("  [成功] 重试成功，JSON 解析正常\n")
				content = retryContent
				parseErr = nil
			} else {
				fmt.Printf("  [警告] 重试后 JSON 仍然解析失败: %v，使用原始响应\n", retryParseErr)
			}
		}
	}

	// 即使 JSON 解析失败，也返回成功，但记录警告
	if parseErr != nil {
		fmt.Printf("  [警告] 最终 JSON 解析失败，但继续处理\n")
	}

	return &ReviewResult{
		FileName:   fileName,
		Content:    content,
		ReviewData: &reviewData,
		Success:    true,
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
