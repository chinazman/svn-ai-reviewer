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

	// 打印完整请求报文
	// fmt.Println("\n  ==================== 完整请求报文 ====================")
	// fmt.Printf("  请求方法: %s\n", req.Method)
	// fmt.Printf("  请求 URL: %s\n", url)
	// fmt.Println("  请求头 (Headers):")
	// for key, values := range req.Header {
	// 	for _, value := range values {
	// 		// 隐藏 API Key 的敏感部分
	// 		if key == "Authorization" && len(value) > 30 {
	// 			fmt.Printf("    %s: %s...%s (已隐藏部分内容)\n", key, value[:15], value[len(value)-5:])
	// 		} else {
	// 			fmt.Printf("    %s: %s\n", key, value)
	// 		}
	// 	}
	// }
	// fmt.Printf("  请求体大小: %d 字节\n", len(jsonData))
	// fmt.Println("  请求体 (Body):")
	// // 格式化 JSON 输出
	// var prettyJSON bytes.Buffer
	// if err := json.Indent(&prettyJSON, jsonData, "    ", "  "); err == nil {
	// 	// 如果请求体太大，只打印前 3000 字符
	// 	prettyStr := prettyJSON.String()
	// 	if len(prettyStr) > 3000 {
	// 		fmt.Printf("    %s\n    ... (已截断，总长度: %d 字节)\n", prettyStr[:3000], len(prettyStr))
	// 	} else {
	// 		fmt.Printf("    %s\n", prettyStr)
	// 	}
	// } else {
	// 	// 如果格式化失败，直接输出原始 JSON
	// 	if len(jsonData) > 3000 {
	// 		fmt.Printf("    %s\n    ... (已截断，总长度: %d 字节)\n", string(jsonData[:3000]), len(jsonData))
	// 	} else {
	// 		fmt.Printf("    %s\n", string(jsonData))
	// 	}
	// }
	// fmt.Println("  ======================================================\n")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		errMsg := fmt.Sprintf("API 请求失败: %v\n请求 URL: %s\n可能原因:\n  1. 网络连接问题\n  2. API 地址配置错误\n  3. 防火墙或代理阻止\n  4. 请求体过大", err, url)
		return &ReviewResult{
			FileName: fileName,
			Success:  false,
			Error:    fmt.Errorf(errMsg),
		}, fmt.Errorf(errMsg)
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
