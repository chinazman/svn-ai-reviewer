# DashScope API 密钥加密说明

## 为什么要加密 API Key？

1. **安全性**：防止 API Key 泄露，避免被他人滥用
2. **版本控制**：加密后的配置文件可以安全地提交到 Git 等版本控制系统
3. **团队协作**：可以在团队中共享配置文件，而不暴露真实的 API Key

## 加密机制

程序使用 AES-256 加密算法对 API Key 进行加密：
- 加密密钥基于机器特征（Machine ID）生成
- 每台机器的加密结果不同
- 配置加载时自动解密

## 使用步骤

### 1. 加密 API Key

```bash
# 语法
svn-ai-reviewer.exe encrypt <your-api-key>

# 示例
svn-ai-reviewer.exe encrypt sk-abc123def456ghi789

# 输出
加密后的 API Key: 0Ati32AZ4NWjIeltiUGDtFUKMOA/F2WQ3dJD4gkhwvmtVoa5h5LqWQ==
```

### 2. 配置文件中使用加密值

将加密后的值填入配置文件：

```yaml
ai:
  provider: "dashscope"
  api_key: "0Ati32AZ4NWjIeltiUGDtFUKMOA/F2WQ3dJD4gkhwvmtVoa5h5LqWQ=="
  model: "your-app-id"
```

### 3. 程序自动解密

运行程序时，会自动解密 API Key：
- 如果解密成功，使用解密后的值
- 如果解密失败，当作明文使用（兼容模式）

## 兼容模式

为了向后兼容，程序支持明文 API Key：

```yaml
ai:
  provider: "dashscope"
  api_key: "sk-abc123def456ghi789"  # 明文（不推荐）
  model: "your-app-id"
```

**警告**：明文 API Key 存在安全风险，不建议在生产环境使用。

## 多机器使用

### 场景 1：单机使用

直接使用加密后的 API Key，无需额外操作。

### 场景 2：多机器共享配置

由于加密密钥基于机器特征，不同机器的加密结果不同：

**方案 A：每台机器独立加密**
```bash
# 机器 A
svn-ai-reviewer.exe encrypt sk-your-api-key
# 输出: EncryptedValueA

# 机器 B
svn-ai-reviewer.exe encrypt sk-your-api-key
# 输出: EncryptedValueB（与机器 A 不同）
```

**方案 B：使用明文（不推荐）**
```yaml
# 所有机器共享同一个明文配置
ai:
  api_key: "sk-your-api-key"
```

**方案 C：环境变量（推荐）**
```bash
# 设置环境变量（未来可能支持）
export DASHSCOPE_API_KEY="sk-your-api-key"
```

## 安全建议

1. **生产环境**：必须使用加密的 API Key
2. **开发环境**：可以使用明文，但不要提交到版本控制
3. **团队协作**：每个成员在自己的机器上加密 API Key
4. **.gitignore**：将包含明文 API Key 的配置文件加入 .gitignore

## 验证加密是否生效

### 方法 1：查看日志

程序启动时不会显示解密过程，但可以通过 API 调用是否成功来验证。

### 方法 2：测试解密

```bash
# 加密
svn-ai-reviewer.exe encrypt sk-test-key

# 将加密值填入配置文件
# 运行程序，如果能正常调用 API，说明解密成功
```

### 方法 3：故意使用错误的加密值

```yaml
ai:
  api_key: "wrong-encrypted-value"
```

如果程序报错 API Key 无效，说明解密机制正常工作。

## 常见问题

### Q1: 加密后的值在其他机器上能用吗？

A: 不能。加密密钥基于机器特征，每台机器需要独立加密。

### Q2: 如何在 CI/CD 中使用？

A: 建议使用环境变量或 CI/CD 平台的密钥管理功能。

### Q3: 忘记原始 API Key 怎么办？

A: 需要到 DashScope 控制台重新生成 API Key。

### Q4: 可以解密已加密的值吗？

A: 目前程序没有提供解密命令，但可以通过查看 DashScope 控制台获取原始 API Key。

### Q5: 加密算法是什么？

A: 使用 AES-256-GCM 加密算法，密钥基于机器 ID 派生。

## 技术细节

### 加密流程

```
原始 API Key
    ↓
获取机器 ID (Machine ID)
    ↓
派生加密密钥 (PBKDF2)
    ↓
AES-256-GCM 加密
    ↓
Base64 编码
    ↓
加密后的 API Key
```

### 解密流程

```
加密后的 API Key
    ↓
Base64 解码
    ↓
获取机器 ID
    ↓
派生解密密钥
    ↓
AES-256-GCM 解密
    ↓
原始 API Key
```

### 代码位置

- 加密/解密实现：`internal/crypto/crypto.go`
- 配置加载：`internal/config/config.go` (第 52-62 行)
- 命令行工具：`cmd/encrypt.go`

## 相关文档

- [更新说明-API密钥加密.md](./更新说明-API密钥加密.md) - 加密功能的更新说明
- [DashScope集成说明.md](./DashScope集成说明.md) - DashScope 完整集成文档
- [DashScope快速测试.md](./DashScope快速测试.md) - 快速测试指南
