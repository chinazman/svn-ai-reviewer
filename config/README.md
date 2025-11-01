# 配置文件目录

此目录用于存放不同的配置文件，方便在多个 AI 模型或配置之间快速切换。

## 使用方法

1. 将配置文件放在此目录下
2. 文件扩展名必须是 `.yaml` 或 `.yml`
3. 刷新审核工具页面，配置文件会自动出现在下拉框中

## 配置文件示例

### deepseek.yaml - DeepSeek 配置

```yaml
ai:
  provider: "deepseek"
  api_key: "your-api-key"
  base_url: "https://api.deepseek.com/v1"
  model: "deepseek-coder"
  temperature: 0.3
  max_tokens: 3000
```

### qwen.yaml - 通义千问配置

```yaml
ai:
  provider: "openai"
  api_key: "your-api-key"
  base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
  model: "qwen-coder-plus"
  temperature: 0.3
  max_tokens: 3000
```

## 注意事项

1. **安全性**：不要将包含真实 API Key 的配置文件提交到版本控制系统
2. **格式**：确保 YAML 格式正确，可以使用在线 YAML 验证工具
3. **命名**：建议使用有意义的文件名，如 `deepseek.yaml`、`gpt4.yaml` 等

## 配置文件结构

完整的配置文件应包含以下部分：

```yaml
# AI 模型配置
ai:
  provider: "openai"      # 提供商：openai, deepseek, custom
  api_key: "your-key"     # API 密钥
  base_url: "https://..."  # API 地址
  model: "model-name"     # 模型名称
  temperature: 0.3        # 温度参数
  max_tokens: 3000        # 最大令牌数

# 审核提示词
review_prompt: |
  你是一个专业的代码审核专家...

# SVN 配置
svn:
  command: "svn"

# 忽略规则
ignore:
  - "*.log"
  - "node_modules/"

# 报告配置
report:
  output_dir: "./reports"
  auto_open: true
```

## 快速开始

1. 复制 `../config.yaml` 作为模板
2. 修改 AI 配置部分
3. 保存为新文件名（如 `myconfig.yaml`）
4. 在审核工具中选择并加载
