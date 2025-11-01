package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"svn-code-reviewer/internal/ai"
	"svn-code-reviewer/internal/config"
	"svn-code-reviewer/internal/report"
	"svn-code-reviewer/internal/svn"
)

var (
	svnURL      string
	svnUsername string
	svnPassword string
	searchPath  string
	searchKeyword string
	searchAuthor string
	saveCredentials bool
)

var onlineCmd = &cobra.Command{
	Use:   "online",
	Short: "在线审核SVN服务器上的指定版本",
	Long:  `连接到SVN服务器，搜索并审核指定版本的代码变更。`,
	RunE:  runOnline,
}

func init() {
	reviewCmd.AddCommand(onlineCmd)
	onlineCmd.Flags().StringVar(&svnURL, "url", "", "SVN服务器地址")
	onlineCmd.Flags().StringVar(&svnUsername, "username", "", "SVN用户名")
	onlineCmd.Flags().StringVar(&svnPassword, "password", "", "SVN密码")
	onlineCmd.Flags().StringVarP(&searchPath, "path", "p", "", "搜索路径（默认根目录）")
	onlineCmd.Flags().StringVarP(&searchKeyword, "keyword", "k", "", "搜索关键词")
	onlineCmd.Flags().StringVarP(&searchAuthor, "author", "a", "", "搜索作者")
	onlineCmd.Flags().BoolVar(&saveCredentials, "save", false, "保存SVN凭据")
}

func runOnline(cmd *cobra.Command, args []string) error {
	// 尝试从配置文件加载凭据
	if svnURL == "" {
		if cfg.Online.URL != "" {
			svnURL = cfg.Online.URL
			svnUsername = cfg.Online.Username
			svnPassword = cfg.Online.Password
			fmt.Println("✓ 已从配置文件加载SVN配置")
		}
	}

	// 如果仍然没有URL，提示用户输入
	if svnURL == "" {
		fmt.Print("请输入SVN服务器地址 (例如: https://svn.example.com/repo 或 file:///path/to/repo): ")
		fmt.Scanln(&svnURL)
	}
	
	// 如果是file://协议，不需要用户名密码
	if !strings.HasPrefix(svnURL, "file://") {
		// 只有非file://协议才提示输入用户名密码
		if svnUsername == "" {
			fmt.Print("请输入SVN用户名 (留空跳过): ")
			fmt.Scanln(&svnUsername)
		}
		if svnPassword == "" && svnUsername != "" {
			fmt.Print("请输入SVN密码 (留空跳过): ")
			fmt.Scanln(&svnPassword)
		}
	}

	// 创建在线SVN客户端
	svnClient := svn.NewOnlineClient(cfg.SVN.Command, svnURL, svnUsername, svnPassword)

	// 测试连接
	fmt.Println("正在测试SVN服务器连接...")
	if err := svnClient.TestConnection(); err != nil {
		return fmt.Errorf("连接SVN服务器失败: %w", err)
	}
	fmt.Println("✓ SVN服务器连接成功")

	// 保存凭据（如果需要）
	if saveCredentials {
		cfg.Online.URL = svnURL
		cfg.Online.Username = svnUsername
		cfg.Online.Password = svnPassword
		if err := config.SaveConfig(cfgFile, cfg); err != nil {
			fmt.Printf("⚠️  保存凭据失败: %v\n", err)
		} else {
			fmt.Println("✓ SVN凭据已保存到配置文件")
		}
	}

	// 搜索日志
	fmt.Println("\n正在搜索SVN提交记录...")
	entries, err := svnClient.SearchLog(searchPath, searchKeyword, searchAuthor, 100, 0)
	if err != nil {
		return fmt.Errorf("搜索日志失败: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("没有找到匹配的提交记录。")
		return nil
	}

	// 显示搜索结果
	fmt.Printf("\n找到 %d 条提交记录:\n", len(entries))
	for i, entry := range entries {
		fmt.Printf("  [%d] r%d | %s | %s\n", i+1, entry.Revision, entry.Author, entry.Date[:19])
		msg := strings.ReplaceAll(entry.Message, "\n", " ")
		if len(msg) > 60 {
			msg = msg[:60] + "..."
		}
		fmt.Printf("      %s\n", msg)
	}

	// 交互式选择版本
	fmt.Println("\n请输入要审核的版本编号（用逗号分隔，例如: 1,3,5）:")
	var input string
	fmt.Print("> ")
	fmt.Scanln(&input)

	if input == "" {
		fmt.Println("未选择任何版本。")
		return nil
	}

	// 解析选择的版本
	var selectedRevisions []int
	indices := strings.Split(input, ",")
	for _, idx := range indices {
		idx = strings.TrimSpace(idx)
		var num int
		_, err := fmt.Sscanf(idx, "%d", &num)
		if err != nil || num < 1 || num > len(entries) {
			fmt.Printf("警告: 忽略无效的编号 '%s'\n", idx)
			continue
		}
		selectedRevisions = append(selectedRevisions, entries[num-1].Revision)
	}

	if len(selectedRevisions) == 0 {
		fmt.Println("没有选择有效的版本。")
		return nil
	}

	// 获取每个版本的文件列表
	var allFiles []svn.FileChange
	for _, rev := range selectedRevisions {
		files, err := svnClient.GetRevisionFiles(rev)
		if err != nil {
			fmt.Printf("⚠️  获取版本 r%d 的文件列表失败: %v\n", rev, err)
			continue
		}
		allFiles = append(allFiles, files...)
	}

	if len(allFiles) == 0 {
		fmt.Println("选中的版本没有文件变更。")
		return nil
	}

	// 显示文件列表
	fmt.Printf("\n检测到 %d 个变更文件:\n", len(allFiles))
	for i, file := range allFiles {
		statusDesc := getStatusDesc(file.Status)
		fmt.Printf("  [%d] %s %s (r%d)\n", i+1, statusDesc, file.Path, file.Revision)
	}

	// 选择要审核的文件
	fmt.Println("\n请输入要审核的文件编号（用逗号分隔，或输入 'all' 审核所有文件）:")
	fmt.Print("> ")
	fmt.Scanln(&input)

	var filesToReview []svn.FileChange
	if input == "all" {
		filesToReview = allFiles
	} else {
		indices := strings.Split(input, ",")
		for _, idx := range indices {
			idx = strings.TrimSpace(idx)
			var num int
			_, err := fmt.Sscanf(idx, "%d", &num)
			if err != nil || num < 1 || num > len(allFiles) {
				fmt.Printf("警告: 忽略无效的编号 '%s'\n", idx)
				continue
			}
			filesToReview = append(filesToReview, allFiles[num-1])
		}
	}

	if len(filesToReview) == 0 {
		fmt.Println("没有选择任何文件进行审核。")
		return nil
	}

	// 创建AI客户端
	aiClient, err := ai.NewClient(&cfg.AI)
	if err != nil {
		return fmt.Errorf("创建AI客户端失败: %w", err)
	}

	// 审核每个文件
	fmt.Printf("\n开始审核 %d 个文件...\n\n", len(filesToReview))
	ctx := context.Background()

	// 创建报告
	htmlReport := &report.Report{
		Title:       "SVN 在线代码审核报告",
		GeneratedAt: time.Now(),
		WorkDir:     svnURL,
		Reviews:     make([]report.FileReview, 0),
	}

	for i, file := range filesToReview {
		fmt.Printf("[%d/%d] 正在审核: %s (r%d)\n", i+1, len(filesToReview), file.Path, file.Revision)

		fileReview := report.FileReview{
			FileName: fmt.Sprintf("%s (r%d)", file.Path, file.Revision),
			Status:   file.Status,
		}

		// 删除的文件直接跳过
		if file.Status == "D" {
			fmt.Printf("  ℹ️  删除的文件，跳过审核\n\n")
			continue
		}

		var diff string
		var err error

		// 对于新增文件，获取完整内容（纯文本，不带diff格式）
		if file.Status == "A" {
			fmt.Printf("  ℹ️  新增文件，获取完整内容\n")
			content, err := svnClient.GetFileContentAtRevision(file.Revision, file.Path)
			if err != nil {
				fmt.Printf("  ⚠️  获取文件内容失败: %v\n", err)
				fmt.Printf("  ℹ️  尝试使用整个版本的diff作为备选\n")
				
				// 备选方案：使用整个版本的diff
				fullDiff, err2 := svnClient.GetRevisionDiff(file.Revision, "")
				if err2 == nil && strings.TrimSpace(fullDiff) != "" {
					diff = fullDiff
					fmt.Printf("  ℹ️  使用整个版本的diff\n\n")
				} else {
					fmt.Printf("  ❌  无法获取文件内容\n\n")
					fileReview.Error = err
					htmlReport.Reviews = append(htmlReport.Reviews, fileReview)
					continue
				}
			} else {
				// 直接使用纯文本内容，不添加任何前缀
				diff = content
			}
		} else {
			// 对于修改的文件，获取diff
			diff, err = svnClient.GetRevisionDiff(file.Revision, file.Path)
			if err != nil {
				fmt.Printf("  ⚠️  获取文件差异失败: %v\n\n", err)
				fileReview.Error = err
				htmlReport.Reviews = append(htmlReport.Reviews, fileReview)
				continue
			}

			if strings.TrimSpace(diff) == "" {
				fmt.Printf("  ⚠️  警告: 未能提取到文件差异内容\n")
				fmt.Printf("      文件路径: %s\n", file.Path)
				fmt.Printf("      这可能是路径匹配问题，将尝试获取整个版本的差异\n\n")
				
				// 尝试获取整个版本的diff作为备选
				fullDiff, err2 := svnClient.GetRevisionDiff(file.Revision, "")
				if err2 == nil && strings.TrimSpace(fullDiff) != "" {
					diff = fullDiff
					fmt.Printf("  ℹ️  使用整个版本的差异内容\n\n")
				} else {
					fmt.Printf("  ℹ️  跳过审核（无差异内容）\n\n")
					continue
				}
			}
		}

		// 调用AI审核
		result, err := aiClient.Review(ctx, file.Path, diff, cfg.ReviewPrompt)
		if err != nil {
			fmt.Printf("  ❌ 审核失败: %v\n\n", err)
			fileReview.Error = err
		} else {
			fmt.Printf("  ✅ 审核完成\n\n")
			fileReview.Result = result
		}

		htmlReport.Reviews = append(htmlReport.Reviews, fileReview)
	}

	// 生成HTML报告
	fmt.Println("正在生成HTML报告...")
	reportPath, err := report.GenerateHTML(htmlReport, cfg.Report.OutputDir)
	if err != nil {
		return fmt.Errorf("生成报告失败: %w", err)
	}

	fmt.Printf("✅ 报告已生成: %s\n", reportPath)

	// 自动打开浏览器
	if cfg.Report.AutoOpen {
		fmt.Println("正在打开浏览器...")
		if err := report.OpenInBrowser(reportPath); err != nil {
			fmt.Printf("⚠️  自动打开浏览器失败: %v\n", err)
			fmt.Printf("请手动打开: %s\n", reportPath)
		}
	}

	fmt.Println("\n所有文件审核完成！")
	return nil
}
