package crypto

import (
	"crypto/des"
	"encoding/base64"
	"fmt"
)

// 写死的 DES 密钥（8字节）
const desKey = "SVN@2025"

// EncryptAPIKey 加密 API Key
func EncryptAPIKey(apiKey string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("API Key 不能为空")
	}

	block, err := des.NewCipher([]byte(desKey))
	if err != nil {
		return "", fmt.Errorf("创建 DES cipher 失败: %w", err)
	}

	// 填充数据到 8 的倍数
	plaintext := []byte(apiKey)
	plaintext = pkcs5Padding(plaintext, block.BlockSize())

	ciphertext := make([]byte, len(plaintext))
	
	// 使用 ECB 模式加密
	for i := 0; i < len(plaintext); i += block.BlockSize() {
		block.Encrypt(ciphertext[i:i+block.BlockSize()], plaintext[i:i+block.BlockSize()])
	}

	// 返回 Base64 编码的密文
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptAPIKey 解密 API Key
func DecryptAPIKey(encryptedKey string) (string, error) {
	if encryptedKey == "" {
		return "", fmt.Errorf("加密的 API Key 不能为空")
	}

	// Base64 解码
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedKey)
	if err != nil {
		return "", fmt.Errorf("Base64 解码失败: %w", err)
	}

	block, err := des.NewCipher([]byte(desKey))
	if err != nil {
		return "", fmt.Errorf("创建 DES cipher 失败: %w", err)
	}

	if len(ciphertext)%block.BlockSize() != 0 {
		return "", fmt.Errorf("密文长度不是块大小的倍数")
	}

	plaintext := make([]byte, len(ciphertext))
	
	// 使用 ECB 模式解密
	for i := 0; i < len(ciphertext); i += block.BlockSize() {
		block.Decrypt(plaintext[i:i+block.BlockSize()], ciphertext[i:i+block.BlockSize()])
	}

	// 去除填充
	plaintext, err = pkcs5Unpadding(plaintext)
	if err != nil {
		return "", fmt.Errorf("去除填充失败: %w", err)
	}

	return string(plaintext), nil
}

// pkcs5Padding PKCS5 填充
func pkcs5Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := make([]byte, padding)
	for i := range padtext {
		padtext[i] = byte(padding)
	}
	return append(data, padtext...)
}

// pkcs5Unpadding PKCS5 去填充
func pkcs5Unpadding(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, fmt.Errorf("数据为空")
	}
	
	padding := int(data[length-1])
	if padding > length || padding > des.BlockSize {
		return nil, fmt.Errorf("无效的填充")
	}
	
	return data[:length-padding], nil
}
