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
	Path   string
	Status string // A=新增, M=修改, D=删除
	Diff   string
}

type Client struct {
	command string
	workDir string
}

func NewClient(command, workDir string) *Client {
	return &Client{
		command: command,
		workDir: workDir,
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
