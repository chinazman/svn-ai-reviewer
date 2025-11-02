package config

import (
	"os"

	"svn-ai-reviewer/internal/crypto"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AI           AIConfig     `yaml:"ai"`
	ReviewPrompt string       `yaml:"review_prompt"`
	SVN          SVNConfig    `yaml:"svn"`
	Ignore       []string     `yaml:"ignore"`
	Report       ReportConfig `yaml:"report"`
	Online       OnlineConfig `yaml:"online"`
}

type AIConfig struct {
	Provider    string  `yaml:"provider"`
	APIKey      string  `yaml:"api_key"`
	BaseURL     string  `yaml:"base_url"`
	Model       string  `yaml:"model"`
	Temperature float32 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
}

type SVNConfig struct {
	Command string `yaml:"command"`
}

type OnlineConfig struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type ReportConfig struct {
	OutputDir  string `yaml:"output_dir"`
	AutoOpen   bool   `yaml:"auto_open"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// 解密 API Key
	if cfg.AI.APIKey != "" {
		decrypted, err := crypto.DecryptAPIKey(cfg.AI.APIKey)
		if err != nil {
			// 如果解密失败，可能是明文，尝试直接使用
			// 这样可以兼容旧的明文配置
			// 但为了安全，可以选择返回错误强制使用加密
			// return nil, fmt.Errorf("解密 API Key 失败: %w", err)
			
			// 兼容模式：解密失败时使用原值（假设是明文）
			decrypted = cfg.AI.APIKey
		}
		cfg.AI.APIKey = decrypted
	}

	// 设置默认值
	if cfg.SVN.Command == "" {
		cfg.SVN.Command = "svn"
	}
	if cfg.Report.OutputDir == "" {
		cfg.Report.OutputDir = "./reports"
	}

	return &cfg, nil
}

func SaveConfig(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
