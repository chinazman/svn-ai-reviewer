package cmd

import (
	"fmt"

	"svn-ai-reviewer/internal/crypto"

	"github.com/spf13/cobra"
)

var encryptCmd = &cobra.Command{
	Use:   "encrypt [api-key]",
	Short: "加密 API Key",
	Long:  `使用 DES 加密算法加密 API Key，加密后的密文可以安全地写入配置文件中。`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		apiKey := args[0]
		
		encrypted, err := crypto.EncryptAPIKey(apiKey)
		if err != nil {
			fmt.Printf("❌ 加密失败: %v\n", err)
			return
		}
		
		fmt.Println("✅ 加密成功！")
		fmt.Println()
		fmt.Println("原始 API Key:")
		fmt.Printf("  %s\n", apiKey)
		fmt.Println()
		fmt.Println("加密后的密文:")
		fmt.Printf("  %s\n", encrypted)
		fmt.Println()
		fmt.Println("请将加密后的密文复制到配置文件的 api_key 字段中。")
	},
}

func init() {
	rootCmd.AddCommand(encryptCmd)
}
