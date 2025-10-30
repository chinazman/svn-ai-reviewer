package svn

import (
	"bytes"
	"fmt"
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

// GetChangedFiles 获取所有变更的文件
func (c *Client) GetChangedFiles() ([]FileChange, error) {
	// 执行 svn status
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

		// 只处理 A(新增), M(修改), D(删除) 状态
		if status == "A" || status == "M" || status == "D" {
			changes = append(changes, FileChange{
				Path:   path,
				Status: status,
			})
		}
	}

	return changes, nil
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

// GetFileContent 获取文件内容（用于新增文件）
func (c *Client) GetFileContent(filePath string) (string, error) {
	absPath := filepath.Join(c.workDir, filePath)
	
	cmd := exec.Command("type", absPath) // Windows
	if strings.Contains(c.command, "/") {
		cmd = exec.Command("cat", absPath) // Unix-like
	}
	
	var out bytes.Buffer
	cmd.Stdout = &out
	
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("读取文件失败: %w", err)
	}
	
	return out.String(), nil
}
