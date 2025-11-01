package svn

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type FileChange struct {
	Path     string
	Status   string // A=新增, M=修改, D=删除
	Diff     string
	Revision int    // 版本号（在线模式使用）
}

type LogEntry struct {
	Revision int
	Author   string
	Date     string
	Message  string
	Paths    []string
}

type Client struct {
	command  string
	workDir  string
	url      string
	username string
	password string
}

func NewClient(command, workDir string) *Client {
	return &Client{
		command: command,
		workDir: workDir,
	}
}

func NewOnlineClient(command, url, username, password string) *Client {
	return &Client{
		command:  command,
		url:      url,
		username: username,
		password: password,
	}
}

// GetChangedFiles 获取所有变更的文件（包括未受控文件）
func (c *Client) GetChangedFiles(ignorePatterns []string) ([]FileChange, error) {
	// 执行 svn status，包括未受控文件
	cmd := exec.Command(c.command, "status")
	cmd.Dir = c.workDir
	
	var out bytes.Buffer
	cmd.Stdout = &out
	
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("执行 svn status 失败: %w", err)
	}

	var changes []FileChange
	lines := strings.Split(out.String(), "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// SVN status 格式: 状态码 文件路径
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		status := parts[0]
		path := parts[len(parts)-1]

		// 处理 A(新增), M(修改), D(删除), ?(未受控) 状态
		if status == "A" || status == "M" || status == "D" || status == "?" {
			// 检查是否应该忽略
			if shouldIgnore(path, ignorePatterns) {
				continue
			}
			
			changes = append(changes, FileChange{
				Path:   path,
				Status: status,
			})
		}
	}

	return changes, nil
}

// shouldIgnore 检查文件路径是否匹配忽略模式
func shouldIgnore(path string, patterns []string) bool {
	for _, pattern := range patterns {
		// 简单的通配符匹配
		if matchPattern(path, pattern) {
			return true
		}
	}
	return false
}

// matchPattern 简单的通配符匹配（支持 * 和目录匹配）
func matchPattern(path, pattern string) bool {
	// 精确匹配
	if path == pattern {
		return true
	}
	
	// 目录匹配（如果模式以 / 结尾）
	if strings.HasSuffix(pattern, "/") {
		if strings.HasPrefix(path, pattern) || strings.HasPrefix(path, strings.TrimSuffix(pattern, "/")) {
			return true
		}
	}
	
	// 通配符匹配
	if strings.Contains(pattern, "*") {
		// 简单实现：将 * 替换为正则表达式
		regexPattern := strings.ReplaceAll(pattern, "*", ".*")
		regexPattern = "^" + regexPattern + "$"
		matched, _ := filepath.Match(pattern, filepath.Base(path))
		if matched {
			return true
		}
		// 检查完整路径
		if strings.Contains(path, strings.ReplaceAll(pattern, "*", "")) {
			return true
		}
	}
	
	// 检查路径中是否包含该模式
	if strings.Contains(path, pattern) {
		return true
	}
	
	return false
}

// GetFileDiff 获取文件的差异内容
func (c *Client) GetFileDiff(filePath string) (string, error) {
	absPath := filepath.Join(c.workDir, filePath)
	
	cmd := exec.Command(c.command, "diff", absPath)
	cmd.Dir = c.workDir
	
	var out bytes.Buffer
	cmd.Stdout = &out
	
	// diff 命令在没有差异时返回非零退出码，这是正常的
	_ = cmd.Run()
	
	return out.String(), nil
}

// GetFileContent 获取文件内容（用于新增文件和未受控文件）
func (c *Client) GetFileContent(filePath string) (string, error) {
	absPath := filepath.Join(c.workDir, filePath)
	
	// 检查文件信息
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("获取文件信息失败: %w", err)
	}
	
	// 检查是否是目录
	if fileInfo.IsDir() {
		return "", fmt.Errorf("路径是目录，不是文件")
	}
	
	// 检查文件大小（限制 10MB）
	const maxFileSize = 10 * 1024 * 1024
	if fileInfo.Size() > maxFileSize {
		return "", fmt.Errorf("文件过大 (%d 字节)，超过 10MB 限制", fileInfo.Size())
	}
	
	// 使用 Go 标准库读取文件，更可靠
	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %w", err)
	}
	
	return string(content), nil
}


// TestConnection 测试SVN服务器连接
func (c *Client) TestConnection() error {
	args := []string{"info", c.url}
	// 只有在提供了用户名时才添加认证参数
	if c.username != "" {
		args = append(args, "--username", c.username)
		if c.password != "" {
			args = append(args, "--password", c.password)
		}
		args = append(args, "--non-interactive")
	}
	
	cmd := exec.Command(c.command, args...)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("连接失败: %w, 错误信息: %s", err, errOut.String())
	}
	
	return nil
}

// SearchLog 搜索SVN日志
func (c *Client) SearchLog(path, keyword, author string, limit, offset int) ([]LogEntry, error) {
	args := []string{"log", c.url}
	
	if path != "" && path != "/" {
		args[1] = c.url + "/" + strings.TrimPrefix(path, "/")
	}
	
	// 只有在提供了用户名时才添加认证信息
	if c.username != "" {
		args = append(args, "--username", c.username)
		if c.password != "" {
			args = append(args, "--password", c.password)
		}
		args = append(args, "--non-interactive")
	}
	
	// 添加限制和搜索条件
	args = append(args, "--limit", fmt.Sprintf("%d", limit))
	if author != "" {
		args = append(args, "--search", author)
	}
	args = append(args, "--verbose")
	args = append(args, "--xml")
	
	cmd := exec.Command(c.command, args...)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("搜索日志失败: %w, 错误信息: %s", err, errOut.String())
	}
	
	entries, err := parseLogXML(out.String())
	if err != nil {
		return nil, fmt.Errorf("解析日志失败: %w", err)
	}
	
	// 如果有关键词过滤
	if keyword != "" {
		var filtered []LogEntry
		for _, entry := range entries {
			if strings.Contains(entry.Message, keyword) || 
			   strings.Contains(entry.Author, keyword) {
				filtered = append(filtered, entry)
			}
		}
		entries = filtered
	}
	
	return entries, nil
}

// GetRevisionDiff 获取指定版本的差异
func (c *Client) GetRevisionDiff(revision int, path string) (string, error) {
	args := []string{"diff", "-c", fmt.Sprintf("%d", revision)}
	
	targetURL := c.url
	if path != "" {
		targetURL = c.url + "/" + strings.TrimPrefix(path, "/")
	}
	args = append(args, targetURL)
	
	// 只有在提供了用户名时才添加认证信息
	if c.username != "" {
		args = append(args, "--username", c.username)
		if c.password != "" {
			args = append(args, "--password", c.password)
		}
		args = append(args, "--non-interactive")
	}
	
	cmd := exec.Command(c.command, args...)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	
	if err := cmd.Run(); err != nil {
		// diff命令在有差异时可能返回非零，这是正常的
		if out.Len() > 0 {
			return out.String(), nil
		}
		return "", fmt.Errorf("获取版本差异失败: %w, 错误信息: %s", err, errOut.String())
	}
	
	return out.String(), nil
}

// GetRevisionFiles 获取指定版本修改的文件列表
func (c *Client) GetRevisionFiles(revision int) ([]FileChange, error) {
	args := []string{"log", c.url, "-r", fmt.Sprintf("%d", revision), "--verbose", "--xml"}
	
	// 只有在提供了用户名时才添加认证信息
	if c.username != "" {
		args = append(args, "--username", c.username)
		if c.password != "" {
			args = append(args, "--password", c.password)
		}
		args = append(args, "--non-interactive")
	}
	
	cmd := exec.Command(c.command, args...)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("获取版本文件列表失败: %w, 错误信息: %s", err, errOut.String())
	}
	
	entries, err := parseLogXML(out.String())
	if err != nil {
		return nil, fmt.Errorf("解析日志失败: %w", err)
	}
	
	if len(entries) == 0 {
		return nil, fmt.Errorf("未找到版本 %d", revision)
	}
	
	var changes []FileChange
	for _, path := range entries[0].Paths {
		// 解析路径格式: "A /path/to/file" 或 "M /path/to/file"
		parts := strings.Fields(path)
		if len(parts) >= 2 {
			changes = append(changes, FileChange{
				Path:     parts[1],
				Status:   parts[0],
				Revision: revision,
			})
		}
	}
	
	return changes, nil
}

// parseLogXML 解析SVN log的XML输出
func parseLogXML(xmlData string) ([]LogEntry, error) {
	// 简单的XML解析实现
	var entries []LogEntry
	
	// 按<logentry>标签分割
	logEntries := strings.Split(xmlData, "<logentry")
	
	for _, entryStr := range logEntries[1:] { // 跳过第一个空元素
		var entry LogEntry
		
		// 提取revision
		if revMatch := extractXMLAttr(entryStr, "revision"); revMatch != "" {
			fmt.Sscanf(revMatch, "%d", &entry.Revision)
		}
		
		// 提取author
		entry.Author = extractXMLTag(entryStr, "author")
		
		// 提取date
		entry.Date = extractXMLTag(entryStr, "date")
		
		// 提取message
		entry.Message = extractXMLTag(entryStr, "msg")
		
		// 提取paths
		pathsSection := extractXMLSection(entryStr, "paths")
		pathEntries := strings.Split(pathsSection, "<path")
		for _, pathStr := range pathEntries[1:] {
			action := extractXMLAttr(pathStr, "action")
			path := extractXMLTagContent(pathStr, "path")
			if path != "" {
				entry.Paths = append(entry.Paths, action+" "+path)
			}
		}
		
		entries = append(entries, entry)
	}
	
	return entries, nil
}

func extractXMLAttr(str, attrName string) string {
	start := strings.Index(str, attrName+"=\"")
	if start == -1 {
		return ""
	}
	start += len(attrName) + 2
	end := strings.Index(str[start:], "\"")
	if end == -1 {
		return ""
	}
	return str[start : start+end]
}

func extractXMLTag(str, tagName string) string {
	startTag := "<" + tagName + ">"
	endTag := "</" + tagName + ">"
	start := strings.Index(str, startTag)
	if start == -1 {
		return ""
	}
	start += len(startTag)
	end := strings.Index(str[start:], endTag)
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(str[start : start+end])
}

func extractXMLSection(str, tagName string) string {
	startTag := "<" + tagName + ">"
	endTag := "</" + tagName + ">"
	start := strings.Index(str, startTag)
	if start == -1 {
		return ""
	}
	end := strings.Index(str[start:], endTag)
	if end == -1 {
		return ""
	}
	return str[start : start+end+len(endTag)]
}

func extractXMLTagContent(str, tagName string) string {
	start := strings.Index(str, ">")
	if start == -1 {
		return ""
	}
	start++
	end := strings.Index(str[start:], "</"+tagName)
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(str[start : start+end])
}
