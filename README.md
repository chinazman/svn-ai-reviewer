# SVN 代码审核工具

一个基于 AI 的 SVN 代码变更审核工具，支持自动检测变更并提供改进建议。

## 功能特性

- ✅ 自动扫描 SVN 工作目录的变更文件（包括新增、修改、删除）
- ✅ 支持选择多个文件进行审核
- ✅ 集成 AI 模型进行智能代码审核
- ✅ 支持 OpenAI 协议的模型（OpenAI、DeepSeek 等）
- ✅ 可配置的审核规则和系统提示词
- ✅ 易于扩展其他 AI 协议

## 安装

### 前置要求

- Go 1.21 或更高版本
- SVN 命令行工具

### 构建

```bash
go mod download
go build -o svn-reviewer.exe
```

## 配置

编辑 `config.yaml` 文件配置 AI 模型和审核规则：

```yaml
ai:
  provider: "deepseek"  # 或 "openai"
  api_key: "your-api-key-here"
  base_url: "https://api.deepseek.com/v1"
  model: "deepseek-chat"
  temperature: 0.3
  max_tokens: 2000

review_prompt: |
  你是一个专业的代码审核专家...
  （自定义审核规则）

svn:
  command: "svn"  # SVN 命令路径
```

## 使用方法

### 基本用法

审核当前目录的所有变更：

```bash
svn-reviewer review
```

### 指定工作目录

```bash
svn-reviewer review -d /path/to/svn/repo
```

### 选择特定文件审核

```bash
svn-reviewer review -f file1.go,file2.go
```

### 交互式选择文件

```bash
svn-reviewer review -i
```

### 使用自定义配置文件

```bash
svn-reviewer review --config custom-config.yaml
```

## 示例输出

```
正在扫描 SVN 变更...

检测到 3 个变更文件:
  [1] [修改] src/main.go
  [2] [新增] src/utils.go
  [3] [修改] README.md

开始审核 3 个文件...

[1/3] 正在审核: src/main.go
  ✅ 审核完成
  ------------------------------------------------------------
  审核结果：
  
  严重程度：中
  
  问题描述：
  1. 缺少错误处理...
  2. 变量命名不够清晰...
  
  改进建议：
  1. 添加适当的错误处理...
  2. 使用更具描述性的变量名...
  ------------------------------------------------------------

所有文件审核完成！
```

## 扩展其他 AI 协议

要添加新的 AI 提供商，只需：

1. 在 `internal/ai/` 目录下创建新的客户端实现
2. 实现 `Client` 接口
3. 在 `NewClient` 函数中添加新的 provider 类型

示例：

```go
// internal/ai/custom.go
type CustomAIClient struct {
    // 自定义字段
}

func (c *CustomAIClient) Review(ctx context.Context, fileName, diff, systemPrompt string) (*ReviewResult, error) {
    // 实现审核逻辑
}
```

## 项目结构

```
svn-code-reviewer/
├── cmd/                    # 命令行接口
│   ├── root.go            # 根命令
│   └── review.go          # 审核命令
├── internal/              # 内部包
│   ├── ai/               # AI 客户端
│   │   ├── client.go     # 客户端接口
│   │   └── openai.go     # OpenAI 实现
│   ├── config/           # 配置管理
│   │   └── config.go
│   └── svn/              # SVN 操作
│       └── svn.go
├── main.go               # 程序入口
├── config.yaml           # 配置文件
└── go.mod               # Go 模块定义
```

## 许可证

MIT
