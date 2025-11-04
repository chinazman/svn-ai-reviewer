package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"svn-ai-reviewer/internal/config"
)

type DashScopeClient struct {
	apiKey     string
	baseURL    string
	appID      string
	httpClient *http.Client
}

type dashScopeRequest struct {
	Input      dashScopeInput      `json:"input"`
	Parameters dashScopeParameters `json:"parameters"`
	Debug      map[string]any      `json:"debug"`
}

type dashScopeInput struct {
	Prompt string `json:"prompt"`
}

type dashScopeParameters struct {
	// 可以添加其他参数，如 temperature 等
}

type dashScopeResponse struct {
	Output struct {
		FinishReason string `json:"finish_reason"`
		SessionID    string `json:"session_id"`
		Text         string `json:"text"`
	} `json:"output"`
	Usage struct {
		Models []struct {
			OutputTokens int    `json:"output_tokens"`
			ModelID      string `json:"model_id"`
			InputTokens  int    `json:"input_tokens"`
		} `json:"models"`
	} `json:"usage"`
	RequestID string `json:"request_id"`
}

func NewDashScopeClient(cfg *config.AIConfig) *DashScopeClient {
	// 从 model 字段提取 app_id
	// 格式: dashscope:app_id 或直接使用 model 作为 app_id
	appID := cfg.Model
	if strings.HasPrefix(appID, "dashscope:") {
		appID = strings.TrimPrefix(appID, "dashscope:")
	}

	// 使用配置的 base_url，如果未配置则使用默认值
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com"
	}

	return &DashScopeClient{
		apiKey:     cfg.APIKey,
		baseURL:    baseURL,
		appID:      appID,
		httpClient: &http.Client{},
	}
}

func (c *DashScopeClient) Review(ctx context.Context, fileName, diff, systemPrompt string) (*ReviewResult, error) {
	// 检查文件内容大小，避免请求过大
	const maxDiffSize = 50000 // 50KB 限制
	if len(diff) > maxDiffSize {
		truncated := diff[:maxDiffSize]
		diff = truncated + fmt.Sprintf("\n\n... (内容过长，已截断，原始大小: %d 字节)", len(diff))
	}

	// 构建完整的 prompt，包含系统提示词和用户内容
	fullPrompt := fmt.Sprintf("%s\n\n文件名: %s\n\n代码变更:\n```\n%s\n```\n\n请审核以上代码变更。",
		systemPrompt, fileName, diff)

	reqBody := dashScopeRequest{
		Input: dashScopeInput{
			Prompt: fullPrompt,
		},
		Parameters: dashScopeParameters{},
		Debug:      map[string]any{},
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

func (c *DashScopeClient) makeRequest(ctx context.Context, jsonData []byte) (string, error) {
	url := fmt.Sprintf("%s/api/v1/apps/%s/completion", c.baseURL, c.appID)
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

	var dashResp dashScopeResponse
	if err := json.Unmarshal(body, &dashResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w\n原始响应:\n%s", err, string(body))
	}

	if dashResp.Output.Text == "" {
		respJSON, _ := json.MarshalIndent(dashResp, "", "  ")
		return "", fmt.Errorf("AI 返回空响应\n完整响应对象:\n%s", string(respJSON))
	}

	return dashResp.Output.Text, nil
}
