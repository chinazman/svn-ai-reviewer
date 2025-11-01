# SVN 代码审核工具

一个基于 AI 的 SVN 代码变更审核工具，支持自动检测变更并提供改进建议。提供图形界面和命令行两种使用方式。

## 功能特性

- ✅ Web 图形界面（GUI）和命令行（CLI）双模式
- ✅ 无需安装额外依赖，纯 Go 实现
- ✅ 自动扫描 SVN 工作目录的变更文件（包括新增、修改、删除）
- ✅ 支持选择多个文件进行审核
- ✅ 集成 AI 模型进行智能代码审核
- ✅ 支持 OpenAI 协议的模型（OpenAI、DeepSeek 等）
- ✅ 可配置的审核规则和系统提示词
- ✅ 生成 HTML 格式的审核报告
- ✅ 易于扩展其他 AI 协议

## 安装

### 前置要求

- Go 1.21 或更高版本
- SVN 命令行工具

### 构建

Windows:
```bash
build.bat
```

或手动构建：
```bash
go mod tidy
go build -o svn-reviewer.exe
```

Linux/Mac:
```bash
go mod tidy
go build -o svn-reviewer
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

### Web GUI 模式（推荐）

直接双击 `svn-reviewer.exe` 启动 Web 界面，程序会自动在浏览器中打开 `http://localhost:8080`：

1. 自动加载配置文件（默认 config.yaml）
2. 输入 SVN 工作目录（默认当前目录）
3. 点击"扫描变更"按钮
4. 在文件列表中选择要审核的文件（默认全选）
5. 点击"开始审核"按钮
6. 等待审核完成，自动生成并打开 HTML 报告

界面特点：
- 🎨 现代化的渐变色设计
- 📱 响应式布局，支持各种屏幕尺寸
- 🔄 实时日志输出
- ✅ 文件状态标识（新增/修改/删除）
- 🚀 一键操作，简单易用

### CLI 模式

#### 基本用法

审核当前目录的所有变更：

```bash
svn-reviewer review
```

#### 指定工作目录

```bash
svn-reviewer review -d /path/to/svn/repo
```

#### 选择特定文件审核

```bash
svn-reviewer review -f file1.go,file2.go
```

#### 交互式选择文件

```bash
svn-reviewer review -i
```

#### 使用自定义配置文件

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
├── gui/                   # 图形界面
│   └── app.go            # GUI 应用
├── internal/              # 内部包
│   ├── ai/               # AI 客户端
│   │   ├── client.go     # 客户端接口
│   │   ├── openai.go     # OpenAI 实现
│   │   └── types.go      # 类型定义
│   ├── config/           # 配置管理
│   │   └── config.go
│   ├── report/           # 报告生成
│   │   └── html.go
│   └── svn/              # SVN 操作
│       └── svn.go
├── main.go               # 程序入口
├── config.yaml           # 配置文件
├── build.bat             # Windows 构建脚本
└── go.mod               # Go 模块定义
```

## 许可证

MIT
