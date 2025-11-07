package cmd

import (
	"github.com/spf13/cobra"
)

var sourceCmd = &cobra.Command{
	Use:   "source",
	Short: "源代码审核模式",
	Long:  `直接输入目录或文件路径进行代码审核`,
	Run: func(cmd *cobra.Command, args []string) {
		// 源代码模式通过GUI界面操作
	},
}

func init() {
	rootCmd.AddCommand(sourceCmd)
}
