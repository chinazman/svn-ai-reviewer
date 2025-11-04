package ai

import (
	"context"
	"fmt"

	"svn-ai-reviewer/internal/config"
)

// ReviewResult AI 审核结果
type ReviewResult struct {
	FileName   string
	Content    string      // 原始返回内容
	ReviewData *ReviewJSON // 解析后的 JSON 数据
	Success    bool
	Error      error
}

// Client AI 客户端接口
type Client interface {
	Review(ctx context.Context, fileName, diff, systemPrompt string) (*ReviewResult, error)
}

// NewClient 根据配置创建 AI 客户端
func NewClient(cfg *config.AIConfig) (Client, error) {
	switch cfg.Provider {
	case "openai", "deepseek", "custom":
		return NewOpenAIClient(cfg), nil
	case "dashscope":
		return NewDashScopeClient(cfg), nil
	default:
		return nil, fmt.Errorf("不支持的 AI 提供商: %s", cfg.Provider)
	}
}
