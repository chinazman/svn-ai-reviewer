package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"svn-ai-reviewer/internal/ai"
	"svn-ai-reviewer/internal/report"
	"svn-ai-reviewer/internal/svn"
)

var (
	workDir       string
	selectedFiles []string
	interactive   bool
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "审核 SVN 代码变更",
	Long:  `扫描 SVN 工作目录的变更文件，并使用 AI 进行代码审核。`,
	RunE:  runReview,
}

func init() {
	rootCmd.AddCommand(reviewCmd)
	reviewCmd.Flags().StringVarP(&workDir, "dir", "d", ".", "SVN 工作目录路径")
	reviewCmd.Flags().StringSliceVarP(&selectedFiles, "files", "f", nil, "指定要审核的文件（逗号分隔）")
	reviewCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "交互式选择文件")
}

func runReview(cmd *cobra.Command, args []string) error {
	// 创建 SVN 客户端
	svnClient := svn.NewClient(cfg.SVN.Command, workDir)

	// 获取变更文件
	fmt.Println("正在扫描 SVN 变更...")
	changes, err := svnClient.GetChangedFiles(cfg.Ignore)
	if err != nil {
		return fmt.Errorf("获取变更文件失败: %w", err)
	}

	if len(changes) == 0 {
		fmt.Println("没有检测到任何变更文件。")
		return nil
	}

	// 显示变更文件列表
	fmt.Printf("\n检测到 %d 个变更文件:\n", len(changes))
	for i, change := range changes {
		statusDesc := getStatusDesc(change.Status)
		fmt.Printf("  [%d] %s %s\n", i+1, statusDesc, change.Path)
	}

	// 选择要审核的文件
	var filesToReview []svn.FileChange
	if len(selectedFiles) > 0 {
		// 使用命令行指定的文件
		filesToReview = filterFilesByNames(changes, selectedFiles)
	} else if interactive {
		// 交互式选择
		filesToReview, err = selectFilesInteractive(changes)
		if err != nil {
			return err
		}
	} else {
		// 默认审核所有文件
		filesToReview = changes
	}

	if len(filesToReview) == 0 {
		fmt.Println("没有选择任何文件进行审核。")
		return nil
	}

	// 创建 AI 客户端
	aiClient, err := ai.NewClient(&cfg.AI)
	if err != nil {
		return fmt.Errorf("创建 AI 客户端失败: %w", err)
	}

	// 审核每个文件
	fmt.Printf("\n开始审核 %d 个文件...\n\n", len(filesToReview))
	ctx := context.Background()

	// 创建报告
	htmlReport := &report.Report{
		Title:       "SVN 代码审核报告",
		GeneratedAt: time.Now(),
		WorkDir:     workDir,
		Reviews:     make([]report.FileReview, 0),
	}

	for i, change := range filesToReview {
		fmt.Printf("[%d/%d] 正在审核: %s\n", i+1, len(filesToReview), change.Path)

		fileReview := report.FileReview{
			FileName: change.Path,
			Status:   change.Status,
		}

		// 获取文件差异
		var diff string
		var skipReview bool
		if change.Status == "D" {
			// 删除的文件，只显示删除信息
			diff = fmt.Sprintf("文件已删除: %s", change.Path)
		} else if change.Status == "A" || change.Status == "?" {
			// 新增文件或未受控文件，获取完整内容
			content, err := svnClient.GetFileContent(change.Path)
			if err != nil {
				fmt.Printf("  ⚠️  获取文件内容失败: %v\n\n", err)
				fileReview.Error = err
				htmlReport.Reviews = append(htmlReport.Reviews, fileReview)
				continue
			}
			statusDesc := "新增文件"
			if change.Status == "?" {
				statusDesc = "未受控文件（尚未加入版本控制）"
			}
			diff = fmt.Sprintf("%s，完整内容:\n%s", statusDesc, content)
		} else {
			// 修改的文件，获取 diff
			d, err := svnClient.GetFileDiff(change.Path)
			if err != nil {
				fmt.Printf("  ⚠️  获取文件差异失败: %v\n\n", err)
				fileReview.Error = err
				htmlReport.Reviews = append(htmlReport.Reviews, fileReview)
				continue
			}
			if strings.TrimSpace(d) == "" {
				fmt.Printf("  ℹ️  文件无差异内容\n\n")
				skipReview = true
			}
			diff = d
		}

		if strings.TrimSpace(diff) == "" || skipReview {
			fmt.Printf("  ℹ️  文件无差异内容，跳过审核\n\n")
			continue
		}

		// 调用 AI 审核
		result, err := aiClient.Review(ctx, change.Path, diff, cfg.ReviewPrompt)
		if err != nil {
			fmt.Printf("  ❌ 审核失败: %v\n\n", err)
			fileReview.Error = err
		} else {
			fmt.Printf("  ✅ 审核完成\n\n")
			fileReview.Result = result
		}

		htmlReport.Reviews = append(htmlReport.Reviews, fileReview)
	}

	// 生成 HTML 报告
	fmt.Println("正在生成 HTML 报告...")
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

func getStatusDesc(status string) string {
	switch status {
	case "A":
		return "[新增]"
	case "M":
		return "[修改]"
	case "D":
		return "[删除]"
	case "?":
		return "[未受控]"
	default:
		return "[" + status + "]"
	}
}

func filterFilesByNames(changes []svn.FileChange, names []string) []svn.FileChange {
	var result []svn.FileChange
	for _, change := range changes {
		for _, name := range names {
			if strings.Contains(change.Path, name) {
				result = append(result, change)
				break
			}
		}
	}
	return result
}

func selectFilesInteractive(changes []svn.FileChange) ([]svn.FileChange, error) {
	fmt.Println("\n请输入要审核的文件编号（用逗号分隔，例如: 1,3,5）或输入 'all' 审核所有文件:")
	
	var input string
	fmt.Print("> ")
	_, err := fmt.Scanln(&input)
	if err != nil {
		return nil, err
	}

	input = strings.TrimSpace(input)
	if input == "all" {
		return changes, nil
	}

	var selected []svn.FileChange
	indices := strings.Split(input, ",")
	
	for _, idx := range indices {
		idx = strings.TrimSpace(idx)
		var num int
		_, err := fmt.Sscanf(idx, "%d", &num)
		if err != nil || num < 1 || num > len(changes) {
			fmt.Printf("警告: 忽略无效的编号 '%s'\n", idx)
			continue
		}
		selected = append(selected, changes[num-1])
	}

	return selected, nil
}

func indentText(text, indent string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}
