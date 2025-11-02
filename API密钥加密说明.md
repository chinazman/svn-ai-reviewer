# API 密钥加密功能说明

## 功能概述

为了提高配置文件的安全性，程序现在支持对 API Key 进行 DES 加密。配置文件中可以填写加密后的密文，程序运行时会自动解密使用。

## 使用方法

### 1. 加密 API Key

使用 `encrypt` 命令加密你的 API Key：

```bash
svn-ai-reviewer.exe encrypt <your-api-key>
```

**示例：**

```bash
svn-ai-reviewer.exe encrypt sk-f759c9fdf70649cb8e58665edb34dd81
```

**输出：**

```
✅ 加密成功！

原始 API Key:
  sk-f759c9fdf70649cb8e58665edb34dd81

加密后的密文:
  0Ati32AZ4NWjIeltiUGDtFUKMOA/F2WQ3dJD4gkhwvmtVoa5h5LqWQ==

请将加密后的密文复制到配置文件的 api_key 字段中。
```

### 2. 更新配置文件

将加密后的密文复制到配置文件的 `api_key` 字段：

```yaml
ai:
  provider: "deepseek"
  api_key: "0Ati32AZ4NWjIeltiUGDtFUKMOA/F2WQ3dJD4gkhwvmtVoa5h5LqWQ=="  # 加密后的密文
  base_url: "https://api.deepseek.com/v1"
  model: "deepseek-chat"
```

### 3. 正常使用

程序运行时会自动解密 API Key，无需任何额外操作：

```bash
# 在线模式
svn-ai-reviewer.exe online

# 审核模式
svn-ai-reviewer.exe review -r 12345
```

## 技术细节

- **加密算法**：DES（Data Encryption Standard）
- **加密模式**：ECB
- **填充方式**：PKCS5
- **编码方式**：Base64
- **密钥**：程序内置固定密钥

## 兼容性说明

程序支持向后兼容：
- 如果配置文件中的 `api_key` 是加密后的密文，程序会自动解密
- 如果配置文件中的 `api_key` 是明文（旧版本配置），程序也能正常使用
- 建议尽快将明文 API Key 加密后更新到配置文件中

## 安全建议

1. **不要将明文 API Key 提交到版本控制系统**
2. **使用加密后的密文替换配置文件中的明文 API Key**
3. **定期更换 API Key**
4. **不要在公开场合分享配置文件**

## 示例

### 加密多个 API Key

如果你有多个配置文件（如 `config/deepseek.yaml`、`config/qwen.yaml`），可以分别加密：

```bash
# 加密 DeepSeek API Key
svn-ai-reviewer.exe encrypt sk-xxxxxxxxxxxxx

# 加密 Qwen API Key  
svn-ai-reviewer.exe encrypt sk-yyyyyyyyyyyyy
```

然后分别更新到对应的配置文件中。
