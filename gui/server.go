package gui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"svn-code-reviewer/internal/ai"
	"svn-code-reviewer/internal/config"
	"svn-code-reviewer/internal/report"
	"svn-code-reviewer/internal/svn"
)

//go:embed templates/*
var templates embed.FS

type Server struct {
	cfg        *config.Config
	changes    []svn.FileChange
	logEntries []svn.LogEntry
	svnClient  *svn.Client
	mode       string // "local" or "online"
	logChannel chan string // SSE日志通道
}

func NewServer() *Server {
	return &Server{
		logChannel: make(chan string, 100),
	}
}

func (s *Server) Start() error {
	http.HandleFunc("/", s.handleIndex)
	http.HandleFunc("/online", s.handleOnlineIndex)
	http.HandleFunc("/api/load-config", s.handleLoadConfig)
	http.HandleFunc("/api/scan", s.handleScan)
	http.HandleFunc("/api/review", s.handleReview)
	http.HandleFunc("/api/diff", s.handleDiff) // 查看文件变更
	http.HandleFunc("/api/online/connect", s.handleOnlineConnect)
	http.HandleFunc("/api/online/search", s.handleOnlineSearch)
	http.HandleFunc("/api/online/files", s.handleOnlineFiles)
	http.HandleFunc("/api/online/review", s.handleOnlineReview)
	http.HandleFunc("/api/online/diff", s.handleOnlineDiff) // 在线模式查看变更
	http.HandleFunc("/api/logs", s.handleLogs) // SSE日志流

	addr := "localhost:8080"
	fmt.Printf("🚀 SVN 代码审核工具已启动\n")
	fmt.Printf("📱 本地模式: http://%s\n", addr)
	fmt.Printf("📱 在线模式: http://%s/online\n", addr)
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

	var files []map[string]interface{}
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

		// 自动打开浏览器
		if s.cfg.Report.AutoOpen {
			s.sendLog("正在打开浏览器...")
			report.OpenInBrowser(reportPath)
		}

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
		Author  string `json:"author"`
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

	entries, err := s.svnClient.SearchLog(req.Path, req.Keyword, req.Author, req.Limit, req.Offset)
	if err != nil {
		respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusInternalServerError)
		return
	}

	s.logEntries = entries

	var logs []map[string]interface{}
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

	var files []map[string]interface{}
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

		// 自动打开浏览器
		if s.cfg.Report.AutoOpen {
			s.sendLog("正在打开浏览器...")
			report.OpenInBrowser(reportPath)
		}

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
