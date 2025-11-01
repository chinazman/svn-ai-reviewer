package main

import (
	"fmt"
	"os"

	"svn-code-reviewer/cmd"
	"svn-code-reviewer/gui"
)

func main() {
	// 如果有命令行参数，使用 CLI 模式
	if len(os.Args) > 1 {
		if err := cmd.Execute(); err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// 否则启动 Web GUI 模式
	server := gui.NewServer()
	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "启动服务器失败: %v\n", err)
		os.Exit(1)
	}
}
