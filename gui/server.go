package gui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"svn-ai-reviewer/internal/ai"
	"svn-ai-reviewer/internal/config"
	"svn-ai-reviewer/internal/report"
	"svn-ai-reviewer/internal/svn"
)

//go:embed templates/*
var templates embed.FS

type Server struct {
	cfg         *config.Config
	changes     []svn.FileChange
	logEntries  []svn.LogEntry
	svnClient   *svn.Client
	mode        string // "local", "online" or "source"
	logChannel  chan string // SSE日志通道
	sourceFiles []SourceFile // 源代码模式的文件列表
}

type SourceFile struct {
	Index int    `json:"index"`
	Path  string `json:"path"`
}

func NewServer() *Server {
	return &Server{
		logChannel: make(chan string, 100),
	}
}

func (s *Server) Start() error {
	http.HandleFunc("/", s.handleIndex)
	http.HandleFunc("/online", s.handleOnlineIndex)
	http.HandleFunc("/source", s.handleSourceIndex)
	http.HandleFunc("/api/list-configs", s.handleListConfigs)
	http.HandleFunc("/api/load-config", s.handleLoadConfig)
	http.HandleFunc("/api/scan", s.handleScan)
	http.HandleFunc("/api/review", s.handleReview)
	http.HandleFunc("/api/diff", s.handleDiff) // 查看文件变更
	http.HandleFunc("/api/online/connect", s.handleOnlineConnect)
	http.HandleFunc("/api/online/search", s.handleOnlineSearch)
	http.HandleFunc("/api/online/files", s.handleOnlineFiles)
	http.HandleFunc("/api/online/review", s.handleOnlineReview)
	http.HandleFunc("/api/online/diff", s.handleOnlineDiff) // 在线模式查看变更
	http.HandleFunc("/api/source/scan", s.handleSourceScan)
	http.HandleFunc("/api/source/content", s.handleSourceContent)
	http.HandleFunc("/api/source/review", s.handleSourceReview)
	http.HandleFunc("/api/logs", s.handleLogs) // SSE日志流
	
	// 提供静态文件服务 - 报告目录
	http.Handle("/reports/", http.StripPrefix("/reports/", http.FileServer(http.Dir("reports"))))

	addr := "localhost:8080"
	fmt.Printf("🚀 SVN 代码审核工具已启动\n")
	fmt.Printf("📱 本地模式: http://%s\n", addr)
	fmt.Printf("📱 在线模式: http://%s/online\n", addr)
	fmt.Printf("📱 源代码模式: http://%s/source\n", addr)
	fmt.Printf("📊 报告目录: http://%s/reports/\n", addr)
	fmt.Println("按 Ctrl+C 停止服务器")

	// 自动打开浏览器
	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser("http://" + addr)
	}()

	return http.ListenAndServe(addr, nil)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(templates, "templates/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

func (s *Server) handleOnlineIndex(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(templates, "templates/online.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

func (s *Server) handleSourceIndex(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(templates, "templates/source.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

func (s *Server) handleListConfigs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var configs []string

	// 检查根目录的 config.yaml
	if _, err := os.Stat("config.yaml"); err == nil {
		configs = append(configs, "config.yaml")
	}

	// 检查 config 目录下的所有 yaml 文件
	if entries, err := os.ReadDir("config"); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				name := entry.Name()
				if strings.HasSuffix(strings.ToLower(name), ".yaml") || strings.HasSuffix(strings.ToLower(name), ".yml") {
					configs = append(configs, "config/"+name)
				}
			}
		}
	}

	respondJSON(w, map[string]interface{}{
		"success": true,
		"configs": configs,
	}, http.StatusOK)
}

func (s *Server) handleLoadConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ConfigPath string `json:"config_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusBadRequest)
		return
	}

	cfg, err := config.LoadConfig(req.ConfigPath)
	if err != nil {
		respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusBadRequest)
		return
	}

	s.cfg = cfg
	respondJSON(w, map[string]interface{}{
		"success": true,
		"message": "配置加载成功",
		"config": map[string]interface{}{
			"provider": cfg.AI.Provider,
			"model":    cfg.AI.Model,
		},
	}, http.StatusOK)
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.cfg == nil {
		respondJSON(w, map[string]interface{}{"error": "请先加载配置文件"}, http.StatusBadRequest)
		return
	}

	var req struct {
		WorkDir string `json:"work_dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusBadRequest)
		return
	}

	if req.WorkDir == "" {
		req.WorkDir = "."
	}

	svnClient := svn.NewClient(s.cfg.SVN.Command, req.WorkDir)
	changes, err := svnClient.GetChangedFiles(s.cfg.Ignore)
	if err != nil {
		respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusInternalServerError)
		return
	}

	s.changes = changes

	// 初始化为空数组而不是 nil，确保 JSON 序列化时返回 [] 而不是 null
	files := make([]map[string]interface{}, 0)
	for i, change := range changes {
		files = append(files, map[string]interface{}{
			"index":  i,
			"path":   change.Path,
			"status": change.Status,
		})
	}

	respondJSON(w, map[string]interface{}{
		"success": true,
		"files":   files,
	}, http.StatusOK)
}

func (s *Server) handleReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.cfg == nil {
		respondJSON(w, map[string]interface{}{"error": "请先加载配置文件"}, http.StatusBadRequest)
		return
	}

	var req struct {
		WorkDir string `json:"work_dir"`
		Indices []int  `json:"indices"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusBadRequest)
		return
	}

	if req.WorkDir == "" {
		req.WorkDir = "."
	}

	// 获取选中的文件
	var filesToReview []svn.FileChange
	for _, idx := range req.Indices {
		if idx >= 0 && idx < len(s.changes) {
			filesToReview = append(filesToReview, s.changes[idx])
		}
	}

	if len(filesToReview) == 0 {
		respondJSON(w, map[string]interface{}{"error": "请至少选择一个文件"}, http.StatusBadRequest)
		return
	}

	// 在后台执行审核
	go func() {
		s.sendLog("开始审核 %d 个文件...", len(filesToReview))
		
		svnClient := svn.NewClient(s.cfg.SVN.Command, req.WorkDir)
		aiClient, err := ai.NewClient(&s.cfg.AI)
		if err != nil {
			s.sendLog("❌ 创建AI客户端失败: %v", err)
			return
		}

		ctx := context.Background()
		htmlReport := &report.Report{
			Title:       "SVN 代码审核报告",
			GeneratedAt: time.Now(),
			WorkDir:     req.WorkDir,
			Reviews:     make([]report.FileReview, 0),
		}

		for i, change := range filesToReview {
			s.sendLog("[%d/%d] 正在审核: %s", i+1, len(filesToReview), change.Path)
			fileReview := report.FileReview{
				FileName: change.Path,
				Status:   change.Status,
			}

			var diff string
			var skipReview bool

			if change.Status == "D" {
				diff = fmt.Sprintf("文件已删除: %s", change.Path)
			} else if change.Status == "A" || change.Status == "?" {
				content, err := svnClient.GetFileContent(change.Path)
				if err != nil {
					s.sendLog("  ⚠️  获取文件内容失败: %v", err)
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
				d, err := svnClient.GetFileDiff(change.Path)
				if err != nil {
					s.sendLog("  ⚠️  获取文件差异失败: %v", err)
					fileReview.Error = err
					htmlReport.Reviews = append(htmlReport.Reviews, fileReview)
					continue
				}
				if strings.TrimSpace(d) == "" {
					skipReview = true
				}
				diff = d
			}

			if strings.TrimSpace(diff) == "" || skipReview {
				s.sendLog("  ℹ️  文件无差异内容，跳过审核")
				continue
			}

			// 保存 diff 内容到报告
			fileReview.Diff = diff

			result, err := aiClient.Review(ctx, change.Path, diff, s.cfg.ReviewPrompt)
			if err != nil {
				s.sendLog("  ❌ 审核失败: %v", err)
				fileReview.Error = err
			} else {
				s.sendLog("  ✅ 审核完成")
				fileReview.Result = result
			}

				htmlReport.Reviews = append(htmlReport.Reviews, fileReview)
		}

		// 生成报告
		s.sendLog("正在生成HTML报告...")
		reportPath, err := report.GenerateHTML(htmlReport, s.cfg.Report.OutputDir)
		if err != nil {
			s.sendLog("❌ 生成报告失败: %v", err)
			return
		}

		absPath, _ := filepath.Abs(reportPath)
		s.sendLog("✅ 报告已生成: %s", absPath)

		// 发送报告URL到前端，由前端打开
		// 将文件路径转换为HTTP URL
		reportFileName := filepath.Base(reportPath)
		reportURL := "http://localhost:8080/reports/" + reportFileName
		s.sendLog("REPORT_URL:" + reportURL)

		s.sendLog("所有文件审核完成！")
	}()

	// 立即返回，审核在后台进行
	respondJSON(w, map[string]interface{}{
		"success": true,
		"message": "审核已开始，请查看日志",
	}, http.StatusOK)
}

func respondJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	}
	if err != nil {
		log.Printf("无法自动打开浏览器: %v", err)
	}
}


func (s *Server) handleOnlineConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL      string `json:"url"`
		Username string `json:"username"`
		Password string `json:"password"`
		Save     bool   `json:"save"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusBadRequest)
		return
	}

	// 验证URL是否提供
	if req.URL == "" {
		respondJSON(w, map[string]interface{}{"error": "请提供SVN服务器地址"}, http.StatusBadRequest)
		return
	}

	// 创建在线SVN客户端（用户名密码可以为空，支持file://协议）
	svnClient := svn.NewOnlineClient("svn", req.URL, req.Username, req.Password)
	
	// 测试连接
	if err := svnClient.TestConnection(); err != nil {
		respondJSON(w, map[string]interface{}{"error": "连接失败: " + err.Error()}, http.StatusBadRequest)
		return
	}

	s.svnClient = svnClient
	s.mode = "online"

	// 保存凭据
	if req.Save && s.cfg != nil {
		s.cfg.Online.URL = req.URL
		s.cfg.Online.Username = req.Username
		s.cfg.Online.Password = req.Password
		// 这里可以选择保存到配置文件
	}

	respondJSON(w, map[string]interface{}{
		"success": true,
		"message": "连接成功",
	}, http.StatusOK)
}

func (s *Server) handleOnlineSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.svnClient == nil {
		respondJSON(w, map[string]interface{}{"error": "请先连接SVN服务器"}, http.StatusBadRequest)
		return
	}

	var req struct {
		Path    string `json:"path"`
		Keyword string `json:"keyword"`
		Limit   int    `json:"limit"`
		Offset  int    `json:"offset"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusBadRequest)
		return
	}

	if req.Limit == 0 {
		req.Limit = 100
	}

	// 关键词现在用于搜索提交信息和作者
	entries, hasMore, err := s.svnClient.SearchLog(req.Path, req.Keyword, req.Limit, req.Offset)
	if err != nil {
		respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusInternalServerError)
		return
	}

	s.logEntries = entries

	// 初始化为空数组而不是 nil，确保 JSON 序列化时返回 [] 而不是 null
	logs := make([]map[string]interface{}, 0)
	for i, entry := range entries {
		logs = append(logs, map[string]interface{}{
			"index":    i,
			"revision": entry.Revision,
			"author":   entry.Author,
			"date":     entry.Date,
			"message":  entry.Message,
		})
	}

	respondJSON(w, map[string]interface{}{
		"success": true,
		"logs":    logs,
		"hasMore": hasMore,
		"offset":  req.Offset,
	}, http.StatusOK)
}

func (s *Server) handleOnlineFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.svnClient == nil {
		respondJSON(w, map[string]interface{}{"error": "请先连接SVN服务器"}, http.StatusBadRequest)
		return
	}

	var req struct {
		Revisions []int `json:"revisions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusBadRequest)
		return
	}

	var allFiles []svn.FileChange
	for _, rev := range req.Revisions {
		files, err := s.svnClient.GetRevisionFiles(rev)
		if err != nil {
			continue
		}
		allFiles = append(allFiles, files...)
	}

	s.changes = allFiles

	// 初始化为空数组而不是 nil，确保 JSON 序列化时返回 [] 而不是 null
	files := make([]map[string]interface{}, 0)
	for i, change := range allFiles {
		files = append(files, map[string]interface{}{
			"index":    i,
			"path":     change.Path,
			"status":   change.Status,
			"revision": change.Revision,
		})
	}

	respondJSON(w, map[string]interface{}{
		"success": true,
		"files":   files,
	}, http.StatusOK)
}

func (s *Server) handleOnlineReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.cfg == nil {
		respondJSON(w, map[string]interface{}{"error": "请先加载配置文件"}, http.StatusBadRequest)
		return
	}

	if s.svnClient == nil {
		respondJSON(w, map[string]interface{}{"error": "请先连接SVN服务器"}, http.StatusBadRequest)
		return
	}

	var req struct {
		Indices []int `json:"indices"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusBadRequest)
		return
	}

	// 获取选中的文件
	var filesToReview []svn.FileChange
	for _, idx := range req.Indices {
		if idx >= 0 && idx < len(s.changes) {
			filesToReview = append(filesToReview, s.changes[idx])
		}
	}

	if len(filesToReview) == 0 {
		respondJSON(w, map[string]interface{}{"error": "请至少选择一个文件"}, http.StatusBadRequest)
		return
	}

	// 在后台执行审核
	go func() {
		s.sendLog("开始审核 %d 个文件...", len(filesToReview))
		
		aiClient, err := ai.NewClient(&s.cfg.AI)
		if err != nil {
			s.sendLog("❌ 创建AI客户端失败: %v", err)
			return
		}

		ctx := context.Background()
		htmlReport := &report.Report{
			Title:       "SVN 在线代码审核报告",
			GeneratedAt: time.Now(),
			WorkDir:     "在线审核",
			Reviews:     make([]report.FileReview, 0),
		}

		for i, file := range filesToReview {
			s.sendLog("[%d/%d] 正在审核: %s (r%d)", i+1, len(filesToReview), file.Path, file.Revision)
			fileReview := report.FileReview{
				FileName: fmt.Sprintf("%s (r%d)", file.Path, file.Revision),
				Status:   file.Status,
				Revision: file.Revision,
			}

			// 删除的文件直接跳过
			if file.Status == "D" {
				s.sendLog("  ℹ️  删除的文件，跳过审核")
				continue
			}

			var diff string
			var err error

			// 对于新增文件，获取完整内容（纯文本，不带diff格式）
			if file.Status == "A" {
				s.sendLog("  ℹ️  新增文件，获取完整内容")
				content, err := s.svnClient.GetFileContentAtRevision(file.Revision, file.Path)
				if err != nil {
					s.sendLog("  ⚠️  获取文件内容失败，尝试使用整个版本的diff")
					// 备选方案：使用整个版本的diff
					fullDiff, err2 := s.svnClient.GetRevisionDiff(file.Revision, "")
					if err2 == nil && strings.TrimSpace(fullDiff) != "" {
						diff = fullDiff
					} else {
						s.sendLog("  ❌ 无法获取文件内容")
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
			diff, err = s.svnClient.GetRevisionDiff(file.Revision, file.Path)
			if err != nil {
				fileReview.Error = err
				htmlReport.Reviews = append(htmlReport.Reviews, fileReview)
				continue
			}

			if strings.TrimSpace(diff) == "" {
				// 尝试获取整个版本的diff作为备选
				fullDiff, err2 := s.svnClient.GetRevisionDiff(file.Revision, "")
				if err2 == nil && strings.TrimSpace(fullDiff) != "" {
					diff = fullDiff
				} else {
					// 如果仍然没有diff，跳过但记录到报告中
					fileReview.Error = fmt.Errorf("未能提取到文件差异内容")
					htmlReport.Reviews = append(htmlReport.Reviews, fileReview)
					continue
				}
			}
		}

			// 保存 diff 内容到报告
			fileReview.Diff = diff

			result, err := aiClient.Review(ctx, file.Path, diff, s.cfg.ReviewPrompt)
			if err != nil {
				s.sendLog("  ❌ 审核失败: %v", err)
				fileReview.Error = err
			} else {
				s.sendLog("  ✅ 审核完成")
				fileReview.Result = result
			}

			htmlReport.Reviews = append(htmlReport.Reviews, fileReview)
		}

		// 生成报告
		s.sendLog("正在生成HTML报告...")
		reportPath, err := report.GenerateHTML(htmlReport, s.cfg.Report.OutputDir)
		if err != nil {
			s.sendLog("❌ 生成报告失败: %v", err)
			return
		}

		absPath, _ := filepath.Abs(reportPath)
		s.sendLog("✅ 报告已生成: %s", absPath)

		// 发送报告URL到前端，由前端打开
		// 将文件路径转换为HTTP URL
		reportFileName := filepath.Base(reportPath)
		reportURL := "http://localhost:8080/reports/" + reportFileName
		s.sendLog("REPORT_URL:" + reportURL)

		s.sendLog("所有文件审核完成！")
	}()

	// 立即返回，审核在后台进行
	respondJSON(w, map[string]interface{}{
		"success": true,
		"message": "审核已开始，请查看日志",
	}, http.StatusOK)
}


// handleLogs 处理SSE日志流
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	// 设置SSE响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 创建一个新的日志通道用于这个连接
	logChan := make(chan string, 10)
	
	// 启动一个goroutine来转发日志
	done := make(chan bool)
	go func() {
		for {
			select {
			case msg := <-s.logChannel:
				logChan <- msg
			case <-done:
				return
			case <-r.Context().Done():
				return
			}
		}
	}()

	// 发送日志到客户端
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case msg := <-logChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			close(done)
			return
		}
	}
}

// sendLog 发送日志消息到SSE通道
func (s *Server) sendLog(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	select {
	case s.logChannel <- msg:
	default:
		// 通道满了，丢弃消息
	}
}


// handleDiff 处理本地模式的文件变更查看
func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.cfg == nil {
		respondJSON(w, map[string]interface{}{"error": "请先加载配置文件"}, http.StatusBadRequest)
		return
	}

	var req struct {
		WorkDir string `json:"work_dir"`
		Index   int    `json:"index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusBadRequest)
		return
	}

	if req.Index < 0 || req.Index >= len(s.changes) {
		respondJSON(w, map[string]interface{}{"error": "无效的文件索引"}, http.StatusBadRequest)
		return
	}

	change := s.changes[req.Index]
	svnClient := svn.NewClient(s.cfg.SVN.Command, req.WorkDir)

	var content string

	if change.Status == "D" {
		content = fmt.Sprintf("文件已删除: %s", change.Path)
	} else if change.Status == "A" || change.Status == "?" {
		fileContent, err := svnClient.GetFileContent(change.Path)
		if err != nil {
			respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusInternalServerError)
			return
		}
		statusDesc := "新增文件"
		if change.Status == "?" {
			statusDesc = "未受控文件"
		}
		content = fmt.Sprintf("%s，完整内容:\n\n%s", statusDesc, fileContent)
	} else {
		diff, err := svnClient.GetFileDiff(change.Path)
		if err != nil {
			respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusInternalServerError)
			return
		}
		content = diff
	}

	respondJSON(w, map[string]interface{}{
		"success": true,
		"file":    change.Path,
		"status":  change.Status,
		"content": content,
	}, http.StatusOK)
}

// handleOnlineDiff 处理在线模式的文件变更查看
func (s *Server) handleOnlineDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.svnClient == nil {
		respondJSON(w, map[string]interface{}{"error": "请先连接SVN服务器"}, http.StatusBadRequest)
		return
	}

	var req struct {
		Index int `json:"index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusBadRequest)
		return
	}

	if req.Index < 0 || req.Index >= len(s.changes) {
		respondJSON(w, map[string]interface{}{"error": "无效的文件索引"}, http.StatusBadRequest)
		return
	}

	file := s.changes[req.Index]
	var content string

	if file.Status == "D" {
		content = fmt.Sprintf("文件已删除: %s (r%d)", file.Path, file.Revision)
	} else if file.Status == "A" {
		fileContent, err := s.svnClient.GetFileContentAtRevision(file.Revision, file.Path)
		if err != nil {
			// 备选：使用整个版本的diff
			fullDiff, err2 := s.svnClient.GetRevisionDiff(file.Revision, "")
			if err2 == nil {
				content = fullDiff
			} else {
				respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusInternalServerError)
				return
			}
		} else {
			content = fmt.Sprintf("新增文件，完整内容:\n\n%s", fileContent)
		}
	} else {
		diff, err := s.svnClient.GetRevisionDiff(file.Revision, file.Path)
		if err != nil {
			respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusInternalServerError)
			return
		}
		if strings.TrimSpace(diff) == "" {
			// 尝试整个版本的diff
			fullDiff, err2 := s.svnClient.GetRevisionDiff(file.Revision, "")
			if err2 == nil {
				content = fullDiff
			} else {
				content = "无法获取文件差异"
			}
		} else {
			content = diff
		}
	}

	respondJSON(w, map[string]interface{}{
		"success":  true,
		"file":     file.Path,
		"status":   file.Status,
		"revision": file.Revision,
		"content":  content,
	}, http.StatusOK)
}


// handleSourceScan 处理源代码模式的文件扫描
func (s *Server) handleSourceScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.cfg == nil {
		respondJSON(w, map[string]interface{}{"error": "请先加载配置文件"}, http.StatusBadRequest)
		return
	}

	var req struct {
		Path   string `json:"path"`
		Filter string `json:"filter"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		respondJSON(w, map[string]interface{}{"error": "请提供目录或文件路径"}, http.StatusBadRequest)
		return
	}

	// 扫描文件
	files, err := scanSourceFiles(req.Path, req.Filter)
	if err != nil {
		respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusInternalServerError)
		return
	}

	s.sourceFiles = files
	s.mode = "source"

	// 初始化为空数组
	fileList := make([]map[string]interface{}, 0)
	for _, file := range files {
		fileList = append(fileList, map[string]interface{}{
			"index": file.Index,
			"path":  file.Path,
		})
	}

	respondJSON(w, map[string]interface{}{
		"success": true,
		"files":   fileList,
	}, http.StatusOK)
}

// scanSourceFiles 扫描指定路径下的文件
func scanSourceFiles(path string, filter string) ([]SourceFile, error) {
	var files []SourceFile
	index := 0

	// 检查路径是否存在
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("路径不存在: %v", err)
	}

	// 如果是文件，直接返回
	if !info.IsDir() {
		if matchFilter(path, filter) {
			files = append(files, SourceFile{
				Index: index,
				Path:  path,
			})
		}
		return files, nil
	}

	// 如果是目录，递归扫描
	err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录
		if info.IsDir() {
			return nil
		}

		// 应用过滤器
		if matchFilter(filePath, filter) {
			files = append(files, SourceFile{
				Index: index,
				Path:  filePath,
			})
			index++
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("扫描文件失败: %v", err)
	}

	return files, nil
}

// matchFilter 检查文件是否匹配过滤器
func matchFilter(filePath string, filter string) bool {
	// 如果没有过滤器，匹配所有文件
	if filter == "" {
		return true
	}

	// 将路径分隔符统一为 /
	filePath = filepath.ToSlash(filePath)
	filter = filepath.ToSlash(filter)

	// 简单的通配符匹配
	matched, err := filepath.Match(filter, filepath.Base(filePath))
	if err == nil && matched {
		return true
	}

	// 尝试匹配完整路径
	matched, err = filepath.Match(filter, filePath)
	if err == nil && matched {
		return true
	}

	// 支持多级路径匹配，例如 src/*.go
	if strings.Contains(filter, "/") {
		parts := strings.Split(filter, "/")
		pathParts := strings.Split(filePath, "/")

		// 从后往前匹配
		if len(parts) <= len(pathParts) {
			match := true
			for i := 0; i < len(parts); i++ {
				partIdx := len(parts) - 1 - i
				pathIdx := len(pathParts) - 1 - i

				matched, err := filepath.Match(parts[partIdx], pathParts[pathIdx])
				if err != nil || !matched {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}

	return false
}

// handleSourceContent 处理源代码模式的文件内容查看
func (s *Server) handleSourceContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Index int `json:"index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusBadRequest)
		return
	}

	if req.Index < 0 || req.Index >= len(s.sourceFiles) {
		respondJSON(w, map[string]interface{}{"error": "无效的文件索引"}, http.StatusBadRequest)
		return
	}

	file := s.sourceFiles[req.Index]

	// 读取文件内容
	content, err := os.ReadFile(file.Path)
	if err != nil {
		respondJSON(w, map[string]interface{}{"error": fmt.Sprintf("读取文件失败: %v", err)}, http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
		"success": true,
		"file":    file.Path,
		"content": string(content),
	}, http.StatusOK)
}

// handleSourceReview 处理源代码模式的审核
func (s *Server) handleSourceReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.cfg == nil {
		respondJSON(w, map[string]interface{}{"error": "请先加载配置文件"}, http.StatusBadRequest)
		return
	}

	var req struct {
		Indices []int `json:"indices"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusBadRequest)
		return
	}

	// 获取选中的文件
	var filesToReview []SourceFile
	for _, idx := range req.Indices {
		if idx >= 0 && idx < len(s.sourceFiles) {
			filesToReview = append(filesToReview, s.sourceFiles[idx])
		}
	}

	if len(filesToReview) == 0 {
		respondJSON(w, map[string]interface{}{"error": "请至少选择一个文件"}, http.StatusBadRequest)
		return
	}

	// 在后台执行审核
	go func() {
		s.sendLog("开始审核 %d 个文件...", len(filesToReview))

		aiClient, err := ai.NewClient(&s.cfg.AI)
		if err != nil {
			s.sendLog("❌ 创建AI客户端失败: %v", err)
			return
		}

		ctx := context.Background()
		htmlReport := &report.Report{
			Title:       "源代码审核报告",
			GeneratedAt: time.Now(),
			WorkDir:     "源代码审核",
			Reviews:     make([]report.FileReview, 0),
		}

		for i, file := range filesToReview {
			s.sendLog("[%d/%d] 正在审核: %s", i+1, len(filesToReview), file.Path)
			fileReview := report.FileReview{
				FileName: file.Path,
				Status:   "源代码",
			}

			// 读取文件内容
			content, err := os.ReadFile(file.Path)
			if err != nil {
				s.sendLog("  ❌ 读取文件失败: %v", err)
				fileReview.Error = err
				htmlReport.Reviews = append(htmlReport.Reviews, fileReview)
				continue
			}

			fileContent := string(content)
			if strings.TrimSpace(fileContent) == "" {
				s.sendLog("  ℹ️  文件为空，跳过审核")
				continue
			}

			// 保存文件内容到报告
			fileReview.Diff = fileContent

			// 调用AI审核
			result, err := aiClient.Review(ctx, file.Path, fileContent, s.cfg.ReviewPrompt)
			if err != nil {
				s.sendLog("  ❌ 审核失败: %v", err)
				fileReview.Error = err
			} else {
				s.sendLog("  ✅ 审核完成")
				fileReview.Result = result
			}

			htmlReport.Reviews = append(htmlReport.Reviews, fileReview)
		}

		// 生成报告
		s.sendLog("正在生成HTML报告...")
		reportPath, err := report.GenerateHTML(htmlReport, s.cfg.Report.OutputDir)
		if err != nil {
			s.sendLog("❌ 生成报告失败: %v", err)
			return
		}

		absPath, _ := filepath.Abs(reportPath)
		s.sendLog("✅ 报告已生成: %s", absPath)

		// 发送报告URL到前端
		reportFileName := filepath.Base(reportPath)
		reportURL := "http://localhost:8080/reports/" + reportFileName
		s.sendLog("REPORT_URL:" + reportURL)

		s.sendLog("所有文件审核完成！")
	}()

	// 立即返回，审核在后台进行
	respondJSON(w, map[string]interface{}{
		"success": true,
		"message": "审核已开始，请查看日志",
	}, http.StatusOK)
}
