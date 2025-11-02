# API 密钥加密功能更新说明

## 更新时间
2025-11-02

## 更新内容

### 1. 新增功能

#### 1.1 API Key 加密命令
新增 `encrypt` 命令，用于加密 API Key：

```bash
svn-ai-reviewer.exe encrypt <your-api-key>
```

**功能特点：**
- 使用 DES 加密算法
- 输出 Base64 编码的密文
- 程序内置固定密钥，无需额外配置

#### 1.2 自动解密功能
程序加载配置文件时会自动解密 API Key：
- 如果配置文件中是加密的密文，自动解密后使用
- 如果配置文件中是明文（旧版本），也能正常使用（向后兼容）

### 2. 文件变更

#### 2.1 新增文件
- `internal/crypto/des.go` - DES 加密/解密工具
- `cmd/encrypt.go` - 加密命令实现
- `API密钥加密说明.md` - 使用说明文档

#### 2.2 修改文件
- `internal/config/config.go` - 添加自动解密逻辑
- `config.yaml` - 更新为加密后的密文
- `config.example.yaml` - 添加加密说明
- `config/deepseek.yaml` - 添加加密说明
- `config/qwen.yaml` - 添加加密说明

### 3. 技术实现

#### 3.1 加密算法
- **算法**：DES (Data Encryption Standard)
- **模式**：ECB
- **填充**：PKCS5
- **编码**：Base64
- **密钥**：程序内置固定密钥 "SVN@2025"

#### 3.2 代码结构
```
internal/crypto/
  └── des.go              # 加密/解密实现
      ├── EncryptAPIKey() # 加密函数
      ├── DecryptAPIKey() # 解密函数
      ├── pkcs5Padding()  # PKCS5 填充
      └── pkcs5Unpadding()# PKCS5 去填充

cmd/
  └── encrypt.go          # 加密命令
```

### 4. 使用示例

#### 4.1 加密 API Key
```bash
# 加密 DeepSeek API Key
svn-ai-reviewer.exe encrypt sk-f759c9fdf70649cb8e58665edb34dd81

# 输出：
# ✅ 加密成功！
# 
# 原始 API Key:
#   sk-f759c9fdf70649cb8e58665edb34dd81
# 
# 加密后的密文:
#   0Ati32AZ4NWjIeltiUGDtFUKMOA/F2WQ3dJD4gkhwvmtVoa5h5LqWQ==
```

#### 4.2 更新配置文件
```yaml
ai:
  provider: "deepseek"
  # 使用加密后的密文
  api_key: "0Ati32AZ4NWjIeltiUGDtFUKMOA/F2WQ3dJD4gkhwvmtVoa5h5LqWQ=="
  base_url: "https://api.deepseek.com/v1"
  model: "deepseek-chat"
```

#### 4.3 正常使用
```bash
# 程序会自动解密 API Key，无需额外操作
svn-ai-reviewer.exe review online
```

### 5. 兼容性说明

- ✅ **向后兼容**：支持明文 API Key（旧版本配置）
- ✅ **自动识别**：自动判断是密文还是明文
- ✅ **无缝升级**：现有用户可以继续使用明文配置
- ⚠️ **安全建议**：建议尽快将明文 API Key 加密

### 6. 安全性提升

#### 6.1 问题
- 旧版本：API Key 明文存储在配置文件中
- 风险：配置文件泄露会直接暴露 API Key

#### 6.2 改进
- 新版本：API Key 加密存储
- 优势：即使配置文件泄露，也需要程序才能解密

#### 6.3 注意事项
- 密钥固定在程序中，反编译仍可能获取
- 主要防护场景：配置文件误提交到公开仓库
- 建议：配合 .gitignore 使用，不提交配置文件

### 7. 迁移指南

#### 7.1 新用户
1. 使用 `encrypt` 命令加密 API Key
2. 将密文填入配置文件
3. 正常使用程序

#### 7.2 现有用户
**选项 1：继续使用明文（不推荐）**
- 无需任何操作
- 程序会自动兼容

**选项 2：升级到加密（推荐）**
1. 使用 `encrypt` 命令加密现有 API Key
2. 将配置文件中的明文替换为密文
3. 测试程序是否正常工作

### 8. 测试验证

#### 8.1 加密测试
```bash
# 测试加密功能
svn-ai-reviewer.exe encrypt test-key-123

# 预期输出：Base64 编码的密文
```

#### 8.2 解密测试
```bash
# 1. 修改配置文件使用加密后的密文
# 2. 运行任意命令测试
svn-ai-reviewer.exe review online

# 预期：程序正常运行，能够调用 AI API
```

### 9. 常见问题

#### Q1: 加密后的密文可以在不同机器上使用吗？
A: 可以。密钥是固定的，加密后的密文在任何机器上都能解密。

#### Q2: 如果忘记了原始 API Key 怎么办？
A: 无法从密文反推原始 API Key。建议保存原始 API Key 的备份。

#### Q3: 可以修改加密密钥吗？
A: 可以修改 `internal/crypto/des.go` 中的 `desKey` 常量，但需要重新编译程序。

#### Q4: 旧的明文配置还能用吗？
A: 可以。程序会自动兼容明文配置，但建议升级到加密配置。

### 10. 后续计划

- [ ] 支持自定义加密密钥（通过环境变量）
- [ ] 支持更强的加密算法（如 AES-256）
- [ ] 支持密钥管理工具集成（如 Vault）
- [ ] 添加密钥轮换功能
