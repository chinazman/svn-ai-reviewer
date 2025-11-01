package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AI           AIConfig     `yaml:"ai"`
	ReviewPrompt string       `yaml:"review_prompt"`
	SVN          SVNConfig    `yaml:"svn"`
	Ignore       []string     `yaml:"ignore"`
	Report       ReportConfig `yaml:"report"`
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

	// 设置默认值
	if cfg.SVN.Command == "" {
		cfg.SVN.Command = "svn"
	}
	if cfg.Report.OutputDir == "" {
		cfg.Report.OutputDir = "./reports"
	}

	return &cfg, nil
}
