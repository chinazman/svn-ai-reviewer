package report

import (
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"svn-code-reviewer/internal/ai"
)

type FileReview struct {
	FileName string
	Status   string
	Result   *ai.ReviewResult
	Error    error
}

type Report struct {
	Title       string
	GeneratedAt time.Time
	WorkDir     string
	Reviews     []FileReview
}

type TemplateData struct {
	Title         string
	GeneratedTime string
	WorkDir       string
	TotalFiles    int
	SuccessCount  int
	ErrorCount    int
	AvgScore      int
	Reviews       []FileReviewData
}

type FileReviewData struct {
	FileName    string
	Status      string
	StatusClass string
	StatusText  string
	HasError    bool
	ErrorMsg    string
	HasReview   bool
	Summary     string
	Score       int
	ScoreClass  string
	IsHighRisk  bool
	Issues      []IssueData
}

type IssueData struct {
	Severity      string
	SeverityClass string
	SeverityText  string
	Title         string
	Description   string
	Suggestion    string
}

func GenerateHTML(report *Report, outputDir string) (string, error) {
	// 创建输出目录
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 生成文件名
	timestamp := report.GeneratedAt.Format("20060102_150405")
	filename := fmt.Sprintf("review_report_%s.html", timestamp)
	filepath := filepath.Join(outputDir, filename)

	// 生成 HTML 内容
	html := generateHTMLContent(report)

	// 写入文件
	if err := os.WriteFile(filepath, []byte(html), 0644); err != nil {
		return "", fmt.Errorf("写入报告文件失败: %w", err)
	}

	return filepath, nil
}

func OpenInBrowser(filepath string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", filepath)
	case "darwin":
		cmd = exec.Command("open", filepath)
	default: // linux
		cmd = exec.Command("xdg-open", filepath)
	}

	return cmd.Start()
}

func generateHTMLContent(report *Report) string {
	// 准备模板数据
	data := prepareTemplateData(report)

	var sb strings.Builder

	// HTML 头部
	sb.WriteString(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>代码审核报告</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            background: #f5f5f5;
            padding: 20px;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 30px;
        }
        .header h1 {
            font-size: 28px;
            margin-bottom: 10px;
        }
        .header .meta {
            opacity: 0.9;
            font-size: 14px;
        }
        .summary {
            padding: 20px 30px;
            background: #f8f9fa;
            border-bottom: 1px solid #e9ecef;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .summary-stats {
            display: flex;
            gap: 30px;
        }
        .summary-item {
            font-size: 14px;
        }
        .summary-item strong {
            color: #667eea;
        }
        .toggle-all-btn {
            padding: 8px 20px;
            background: #667eea;
            color: white;
            border: none;
            border-radius: 5px;
            cursor: pointer;
            font-size: 14px;
            font-weight: 500;
            transition: background 0.3s ease;
        }
        .toggle-all-btn:hover {
            background: #5568d3;
        }
        .content {
            padding: 30px;
        }
        .file-list {
            margin-bottom: 20px;
        }
        .file-item {
            background: white;
            border: 1px solid #e9ecef;
            border-radius: 6px;
            margin-bottom: 10px;
            overflow: hidden;
            transition: all 0.3s ease;
        }
        .file-item:hover {
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }
        .file-item.high-risk {
            border-left: 4px solid #dc3545;
        }
        .file-header {
            background: #f8f9fa;
            padding: 15px 20px;
            display: flex;
            align-items: center;
            justify-content: space-between;
            cursor: pointer;
            user-select: none;
        }
        .file-header:hover {
            background: #e9ecef;
        }
        .file-info {
            display: flex;
            align-items: center;
            gap: 15px;
            flex: 1;
        }
        .file-name {
            font-weight: 600;
            font-size: 15px;
            color: #2c3e50;
        }
        .file-badges {
            display: flex;
            align-items: center;
            gap: 10px;
        }
        .expand-icon {
            font-size: 20px;
            color: #6c757d;
            transition: transform 0.3s ease;
        }
        .file-item.expanded .expand-icon {
            transform: rotate(90deg);
        }
        .status-badge {
            display: inline-block;
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 12px;
            font-weight: 500;
        }
        .status-new { background: #d4edda; color: #155724; }
        .status-modified { background: #fff3cd; color: #856404; }
        .status-deleted { background: #f8d7da; color: #721c24; }
        .status-untracked { background: #d1ecf1; color: #0c5460; }
        .file-body {
            padding: 20px;
            display: none;
            border-top: 1px solid #e9ecef;
            background: white;
        }
        .file-item.expanded .file-body {
            display: block;
        }
        .review-content {
            line-height: 1.8;
        }
        .review-content h1, .review-content h2, .review-content h3 {
            margin-top: 20px;
            margin-bottom: 10px;
            color: #2c3e50;
        }
        .review-content h1 { font-size: 24px; }
        .review-content h2 { font-size: 20px; }
        .review-content h3 { font-size: 18px; }
        .review-content ul, .review-content ol {
            margin-left: 20px;
            margin-bottom: 15px;
        }
        .review-content li {
            margin-bottom: 8px;
        }
        .review-content code {
            background: #f4f4f4;
            padding: 2px 6px;
            border-radius: 3px;
            font-family: "Courier New", monospace;
            font-size: 14px;
        }
        .review-content pre {
            background: #f4f4f4;
            padding: 15px;
            border-radius: 5px;
            overflow-x: auto;
            margin: 15px 0;
        }
        .review-content pre code {
            background: none;
            padding: 0;
        }
        .error-message {
            background: #f8d7da;
            color: #721c24;
            padding: 15px;
            border-radius: 5px;
            border-left: 4px solid #f5c6cb;
        }
        .success-icon {
            color: #28a745;
            margin-right: 5px;
        }
        .error-icon {
            color: #dc3545;
            margin-right: 5px;
        }
        .score-badge {
            display: inline-block;
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 13px;
            font-weight: 600;
        }
        .score-high { background: #d4edda; color: #155724; }
        .score-medium { background: #fff3cd; color: #856404; }
        .score-low { background: #f8d7da; color: #721c24; }
        .risk-badge {
            display: inline-block;
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 12px;
            font-weight: 600;
            background: #f8d7da;
            color: #721c24;
        }
        .issue-item {
            margin-bottom: 20px;
            padding: 15px;
            border-left: 4px solid #dee2e6;
            background: #f8f9fa;
            border-radius: 4px;
        }
        .issue-item.severity-high { border-left-color: #dc3545; }
        .issue-item.severity-medium { border-left-color: #ffc107; }
        .issue-item.severity-low { border-left-color: #17a2b8; }
        .issue-title {
            font-weight: 600;
            margin-bottom: 8px;
            color: #2c3e50;
        }
        .issue-desc {
            margin-bottom: 8px;
            color: #495057;
        }
        .issue-suggestion {
            color: #28a745;
            font-style: italic;
        }
        .section-title {
            font-size: 16px;
            font-weight: 600;
            margin: 15px 0 10px 0;
            color: #495057;
        }
        .list-item {
            padding: 5px 0;
            color: #495057;
        }
        .footer {
            text-align: center;
            padding: 20px;
            color: #6c757d;
            font-size: 14px;
            border-top: 1px solid #e9ecef;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>📋 代码审核报告</h1>
            <div class="meta">
                生成时间: ` + data.GeneratedTime + `<br>
                工作目录: ` + html.EscapeString(data.WorkDir) + `
            </div>
        </div>
        <div class="summary">
            <div class="summary-stats">
                <span class="summary-item"><strong>总文件数:</strong> ` + fmt.Sprintf("%d", data.TotalFiles) + `</span>
                <span class="summary-item"><strong>审核成功:</strong> ` + fmt.Sprintf("%d", data.SuccessCount) + `</span>
                <span class="summary-item"><strong>审核失败:</strong> ` + fmt.Sprintf("%d", data.ErrorCount) + `</span>`)

	if data.AvgScore > 0 {
		sb.WriteString(`
                <span class="summary-item"><strong>平均评分:</strong> ` + fmt.Sprintf("%d", data.AvgScore) + `</span>`)
	}

	sb.WriteString(`
            </div>
            <button class="toggle-all-btn" onclick="toggleAll()">全部展开</button>
        </div>
        <div class="content">
            <div class="file-list">
`)

	// 渲染文件列表
	for i, fileData := range data.Reviews {
		highRiskClass := ""
		if fileData.IsHighRisk {
			highRiskClass = " high-risk"
		}

		sb.WriteString(`
                <div class="file-item` + highRiskClass + `" id="file-` + fmt.Sprintf("%d", i) + `">
                    <div class="file-header" onclick="toggleFile(` + fmt.Sprintf("%d", i) + `)">
                        <div class="file-info">
                            <span class="file-name">` + html.EscapeString(fileData.FileName) + `</span>
                            <div class="file-badges">
                                <span class="status-badge status-` + fileData.StatusClass + `">` + fileData.StatusText + `</span>`)

		if fileData.IsHighRisk {
			sb.WriteString(`
                                <span class="risk-badge">⚠️ 高风险</span>`)
		}

		if fileData.HasReview && fileData.Score > 0 {
			sb.WriteString(`
                                <span class="score-badge score-` + fileData.ScoreClass + `">` + fmt.Sprintf("%d分", fileData.Score) + `</span>`)
		}

		sb.WriteString(`
                            </div>
                        </div>
                        <span class="expand-icon">▶</span>
                    </div>
                    <div class="file-body">`)

		if fileData.HasError {
			sb.WriteString(`
                        <div class="error-message">
                            <span class="error-icon">❌</span>
                            <strong>审核失败:</strong> ` + html.EscapeString(fileData.ErrorMsg) + `
                        </div>`)
		} else if fileData.HasReview {
			// 总结
			if fileData.Summary != "" {
				sb.WriteString(`
                        <div class="review-content">
                            <p><strong>📝 总结:</strong> ` + html.EscapeString(fileData.Summary) + `</p>
                        </div>`)
			}

			// 问题列表
			if len(fileData.Issues) > 0 {
				sb.WriteString(`
                        <div class="section-title">⚠️ 发现的问题 (` + fmt.Sprintf("%d", len(fileData.Issues)) + `)</div>`)
				for _, issue := range fileData.Issues {
					sb.WriteString(`
                        <div class="issue-item severity-` + issue.Severity + `">
                            <div class="issue-title">
                                <span class="status-badge status-` + issue.SeverityClass + `">` + issue.SeverityText + `</span>
                                ` + html.EscapeString(issue.Title) + `
                            </div>
                            <div class="issue-desc">` + html.EscapeString(issue.Description) + `</div>
                            <div class="issue-suggestion">💡 建议: ` + html.EscapeString(issue.Suggestion) + `</div>
                        </div>`)
				}
			} else {
				sb.WriteString(`
                        <div class="review-content">
                            <p style="color: #28a745;">✅ 未发现明显问题</p>
                        </div>`)
			}
		}

		sb.WriteString(`
                    </div>
                </div>`)
	}

	sb.WriteString(`
            </div>
        </div>
        <div class="footer">
            由 SVN 代码审核工具生成
        </div>
    </div>
    <script>
        let allExpanded = false;

        function toggleFile(index) {
            const fileItem = document.getElementById('file-' + index);
            fileItem.classList.toggle('expanded');
        }

        function toggleAll() {
            const fileItems = document.querySelectorAll('.file-item');
            const btn = document.querySelector('.toggle-all-btn');
            
            allExpanded = !allExpanded;
            
            fileItems.forEach(item => {
                if (allExpanded) {
                    item.classList.add('expanded');
                } else {
                    item.classList.remove('expanded');
                }
            });
            
            btn.textContent = allExpanded ? '全部收起' : '全部展开';
        }
    </script>
</body>
</html>`)

	return sb.String()
}

func prepareTemplateData(report *Report) *TemplateData {
	data := &TemplateData{
		Title:         report.Title,
		GeneratedTime: report.GeneratedAt.Format("2006-01-02 15:04:05"),
		WorkDir:       report.WorkDir,
		TotalFiles:    len(report.Reviews),
		Reviews:       make([]FileReviewData, 0),
	}

	totalScore := 0
	scoreCount := 0

	for _, review := range report.Reviews {
		fileData := FileReviewData{
			FileName:    review.FileName,
			Status:      review.Status,
			StatusClass: getStatusClass(review.Status),
			StatusText:  getStatusText(review.Status),
		}

		if review.Error != nil {
			fileData.HasError = true
			fileData.ErrorMsg = review.Error.Error()
			data.ErrorCount++
		} else if review.Result != nil && review.Result.Success {
			data.SuccessCount++
			fileData.HasReview = true

			if review.Result.ReviewData != nil {
				rd := review.Result.ReviewData
				fileData.Summary = rd.Summary
				fileData.Score = rd.Score
				fileData.ScoreClass = getScoreClass(rd.Score)

				if rd.Score > 0 {
					totalScore += rd.Score
					scoreCount++
				}

				// 转换问题列表
				hasHighSeverity := false
				for _, issue := range rd.Issues {
					if issue.Severity == "high" {
						hasHighSeverity = true
					}
					fileData.Issues = append(fileData.Issues, IssueData{
						Severity:      issue.Severity,
						SeverityClass: getSeverityClass(issue.Severity),
						SeverityText:  getSeverityText(issue.Severity),
						Title:         issue.Title,
						Description:   issue.Description,
						Suggestion:    issue.Suggestion,
					})
				}

				// 判断是否高风险：分数低于60或有高严重性问题
				fileData.IsHighRisk = rd.Score < 60 || hasHighSeverity
			}
		}

		data.Reviews = append(data.Reviews, fileData)
	}

	if scoreCount > 0 {
		data.AvgScore = totalScore / scoreCount
	}

	return data
}

func getScoreClass(score int) string {
	if score >= 80 {
		return "high"
	} else if score >= 60 {
		return "medium"
	}
	return "low"
}

func getSeverityClass(severity string) string {
	switch severity {
	case "high":
		return "deleted"
	case "medium":
		return "modified"
	case "low":
		return "new"
	default:
		return "modified"
	}
}

func getSeverityText(severity string) string {
	switch severity {
	case "high":
		return "高"
	case "medium":
		return "中"
	case "low":
		return "低"
	default:
		return severity
	}
}

func getStatusClass(status string) string {
	switch status {
	case "A":
		return "new"
	case "M":
		return "modified"
	case "D":
		return "deleted"
	case "?":
		return "untracked"
	default:
		return "modified"
	}
}

func getStatusText(status string) string {
	switch status {
	case "A":
		return "新增"
	case "M":
		return "修改"
	case "D":
		return "删除"
	case "?":
		return "未受控"
	default:
		return status
	}
}


