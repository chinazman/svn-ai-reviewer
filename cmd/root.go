package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"svn-ai-reviewer/internal/config"
)

var (
	cfgFile string
	cfg     *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "svn-reviewer",
	Short: "SVN 代码审核工具",
	Long:  `一个基于 AI 的 SVN 代码变更审核工具，支持自动检测变更并提供改进建议。`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config.yaml", "配置文件路径")
}

func initConfig() {
	var err error
	cfg, err = config.LoadConfig(cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置文件失败: %v\n", err)
		os.Exit(1)
	}
}
