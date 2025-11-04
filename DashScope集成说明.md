# DashScope 集成说明

## 概述

已成功集成阿里云 DashScope 作为新的 AI 模型提供商，支持使用通义千问等模型进行代码审核。

## 实现内容

### 1. 新增文件

- `internal/ai/dashscope.go` - DashScope 客户端实现
- `config/dashscope.yaml` - DashScope 配置示例文件

### 2. 修改文件

- `internal/ai/client.go` - 添加 DashScope 提供商支持
- `config/README.md` - 更新配置文档，添加 DashScope 说明

## DashScope API 特点

DashScope 使用应用级别的 API，与 OpenAI 兼容模式不同：

- **请求地址**: `{BASE_URL}/api/v1/apps/{APP_ID}/completion`
  - 默认 BASE_URL: `https://dashscope.aliyuncs.com`
  - 可在配置文件中自定义
- **认证方式**: Bearer Token（API Key）
- **请求格式**: 
  ```json
  {
    "input": {
      "prompt": "你的提示词"
    },
    "parameters": {},
    "debug": {}
  }
  ```
- **响应格式**:
  ```json
  {
    "output": {
      "finish_reason": "stop",
      "session_id": "...",
      "text": "AI 返回的文本"
    },
    "usage": {...},
    "request_id": "..."
  }
  ```

## 使用方法

### 1. 获取 DashScope 凭证

1. 访问 [阿里云 DashScope 控制台](https://dashscope.console.aliyun.com/)
2. 创建应用并获取应用 ID（APP_ID）
3. 获取 API Key

### 2. 配置文件设置

创建或修改配置文件（如 `config/dashscope.yaml`）：

```yaml
ai:
  provider: "dashscope"
  api_key: "your-dashscope-api-key"  # 支持加密或明文
  base_url: "https://dashscope.aliyuncs.com"  # 可选，默认值
  model: "YOUR_APP_ID"  # 应用 ID
```

**注意**：
- `provider` 必须设置为 `"dashscope"`
- `api_key` 支持加密存储（推荐）或明文存储
- `model` 字段用于存储应用 ID
- `base_url` 可选，默认为 `https://dashscope.aliyuncs.com`
- `temperature`、`max_tokens` 等参数在 DashScope 应用配置中设置

### 3. 加密 API Key（强烈推荐）

为了安全，强烈建议加密 API Key：

```bash
# 加密 API Key
svn-ai-reviewer.exe encrypt sk-your-dashscope-api-key

# 输出示例：
# 加密后的 API Key: 0Ati32AZ4NWjIeltiUGDtFUKMOA/F2WQ3dJD4gkhwvmtVoa5h5LqWQ==

# 将加密后的值填入配置文件
ai:
  api_key: "0Ati32AZ4NWjIeltiUGDtFUKMOA/F2WQ3dJD4gkhwvmtVoa5h5LqWQ=="
```

**加密说明**：
- 配置加载时会自动尝试解密 API Key
- 如果解密失败，会当作明文使用（兼容模式）
- 加密后的配置文件可以安全地提交到版本控制系统

### 4. 使用配置

在 GUI 界面中：
1. 刷新页面
2. 在配置文件下拉框中选择 `dashscope.yaml`
3. 点击"加载配置"
4. 开始审核

## 技术实现

### DashScopeClient 结构

```go
type DashScopeClient struct {
    apiKey     string
    baseURL    string
    appID      string
    httpClient *http.Client
}
```

### 关键方法

- `NewDashScopeClient(cfg *config.AIConfig)` - 创建客户端实例
- `Review(ctx, fileName, diff, systemPrompt)` - 执行代码审核
- `makeRequest(ctx, jsonData)` - 发送 HTTP 请求

### 错误处理

- 支持 JSON 解析失败时自动重试
- 内容过长时自动截断（50KB 限制）
- 详细的错误日志输出

## 与其他提供商的区别

| 特性 | OpenAI/DeepSeek | DashScope |
|------|----------------|-----------|
| API 风格 | Chat Completions | App Completion |
| 认证 | Bearer Token | Bearer Token |
| 模型参数 | 请求中指定 | 应用中配置 |
| 系统提示词 | 独立 message | 合并到 prompt |
| 响应格式 | choices[].message.content | output.text |

## 测试建议

1. 使用简单的代码变更测试基本功能
2. 验证 JSON 解析是否正常
3. 测试重试机制
4. 检查错误处理

## 注意事项

1. **API Key 加密**：强烈建议使用加密的 API Key，配置加载时会自动解密
2. **应用 ID**：DashScope 的应用 ID 是必需的，需要在控制台创建应用
3. **参数配置**：应用级别的参数（如 temperature）在 DashScope 控制台配置，不在配置文件中
4. **提示词合并**：系统提示词会与用户内容合并为一个完整的 prompt
5. **配额检查**：确保 API Key 有足够的配额
6. **兼容模式**：支持明文 API Key（不推荐），但建议使用加密方式

## 后续优化建议

1. 支持更多 DashScope 参数（如果需要）
2. 添加流式响应支持（SSE）
3. 支持多轮对话（使用 session_id）
4. 添加更详细的使用统计
