# DashScope 快速测试指南

## 前置准备

1. 获取 DashScope API Key 和应用 ID
   - 访问：https://dashscope.console.aliyun.com/
   - 创建应用并记录应用 ID

2. 配置文件准备
   - 复制 `config/dashscope.yaml` 作为模板
   - 填入你的 API Key 和应用 ID

## 测试步骤

### 1. 加密 API Key（强烈推荐）

```bash
# 加密你的 DashScope API Key
svn-ai-reviewer.exe encrypt sk-your-dashscope-api-key

# 输出示例：
# 加密后的 API Key: 0Ati32AZ4NWjIeltiUGDtFUKMOA/F2WQ3dJD4gkhwvmtVoa5h5LqWQ==
```

### 2. 配置 API 凭证

编辑 `config/dashscope.yaml`，使用加密后的 API Key：

```yaml
ai:
  provider: "dashscope"
  api_key: "0Ati32AZ4NWjIeltiUGDtFUKMOA/F2WQ3dJD4gkhwvmtVoa5h5LqWQ=="  # 加密后的值
  base_url: "https://dashscope.aliyuncs.com"  # 可选，默认值
  model: "your-app-id"  # 你的应用 ID
```

**注意**：也可以直接使用明文 API Key，但不推荐（安全风险）

### 3. 启动 GUI 服务

```bash
# 使用新编译的程序
svn-ai-reviewer-new.exe online

# 或使用原程序（需要先替换）
svn-ai-reviewer.exe online
```

### 4. 在浏览器中测试

1. 打开 http://localhost:8080
2. 在配置文件下拉框中选择 `dashscope.yaml`
3. 点击"加载配置"按钮
4. 输入 SVN 仓库 URL 或本地路径
5. 输入版本号范围
6. 点击"开始审核"

### 5. 验证结果

检查以下内容：
- [ ] 配置加载成功
- [ ] API 请求正常
- [ ] 返回 JSON 格式正确
- [ ] 审核结果显示正常
- [ ] 评分和问题列表正确

## 常见问题

### 1. API Key 无效

**错误信息**: `API 返回错误状态码: 401`

**解决方法**:
- 检查 API Key 是否正确
- 确认 API Key 是否已激活
- 检查是否有足够的配额

### 2. 应用 ID 错误

**错误信息**: `API 返回错误状态码: 404`

**解决方法**:
- 确认应用 ID 是否正确
- 检查应用是否已创建并启用

### 3. JSON 解析失败

**警告信息**: `[警告] JSON 解析失败`

**说明**:
- 程序会自动重试一次
- 如果仍然失败，会使用原始响应继续处理
- 可能需要调整应用的系统提示词

### 4. 响应为空

**错误信息**: `AI 返回空响应`

**解决方法**:
- 检查应用配置是否正确
- 确认应用是否有输出限制
- 查看 DashScope 控制台的调用日志

## 调试技巧

### 1. 查看详细日志

程序会输出详细的调试信息：
```
[信息] 正在审核文件: xxx.go
[警告] JSON 解析失败: ...
[信息] 正在重试请求...
[成功] 重试成功，JSON 解析正常
```

### 2. 测试 API 连接

使用 curl 测试：
```bash
curl -X POST https://dashscope.aliyuncs.com/api/v1/apps/YOUR_APP_ID/completion \
  --header "Authorization: Bearer YOUR_API_KEY" \
  --header "Content-Type: application/json" \
  --data '{"input": {"prompt": "你好"},"parameters": {},"debug": {}}'
```

### 3. 检查配置加载

在 GUI 中点击"加载配置"后，查看浏览器控制台是否有错误信息。

## 性能优化

1. **内容截断**: 超过 50KB 的文件会自动截断
2. **自动重试**: JSON 解析失败时自动重试一次
3. **错误容忍**: 即使解析失败也会继续处理

## 下一步

测试成功后：
1. 可以将加密后的配置提交到版本控制
2. 配置更多的审核规则
3. 调整应用的系统提示词以获得更好的审核效果
4. 在团队中推广使用

## 反馈

如有问题或建议，请记录：
- 错误信息
- 配置内容（隐藏敏感信息）
- 复现步骤
