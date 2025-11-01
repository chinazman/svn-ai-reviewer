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
	cfg     *config.Config
	changes []svn.FileChange
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Start() error {
	http.HandleFunc("/", s.handleIndex)
	http.HandleFunc("/api/load-config", s.handleLoadConfig)
	http.HandleFunc("/api/scan", s.handleScan)
	http.HandleFunc("/api/review", s.handleReview)

	addr := "localhost:8080"
	fmt.Printf("🚀 SVN 代码审核工具已启动\n")
	fmt.Printf("📱 请在浏览器中打开: http://%s\n", addr)
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

	// 执行审核
	svnClient := svn.NewClient(s.cfg.SVN.Command, req.WorkDir)
	aiClient, err := ai.NewClient(&s.cfg.AI)
	if err != nil {
		respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusInternalServerError)
		return
	}

	ctx := context.Background()
	htmlReport := &report.Report{
		Title:       "SVN 代码审核报告",
		GeneratedAt: time.Now(),
		WorkDir:     req.WorkDir,
		Reviews:     make([]report.FileReview, 0),
	}

	for _, change := range filesToReview {
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
			continue
		}

		result, err := aiClient.Review(ctx, change.Path, diff, s.cfg.ReviewPrompt)
		if err != nil {
			fileReview.Error = err
		} else {
			fileReview.Result = result
		}

		htmlReport.Reviews = append(htmlReport.Reviews, fileReview)
	}

	// 生成报告
	reportPath, err := report.GenerateHTML(htmlReport, s.cfg.Report.OutputDir)
	if err != nil {
		respondJSON(w, map[string]interface{}{"error": err.Error()}, http.StatusInternalServerError)
		return
	}

	absPath, _ := filepath.Abs(reportPath)

	// 自动打开浏览器
	if s.cfg.Report.AutoOpen {
		report.OpenInBrowser(reportPath)
	}

	respondJSON(w, map[string]interface{}{
		"success":     true,
		"report_path": absPath,
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
