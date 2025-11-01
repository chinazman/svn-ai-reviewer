# SSE实时日志实现说明

## 已完成的后端实现

### 1. 添加SSE支持

**服务器结构：**
```go
type Server struct {
    logChannel chan string // SSE日志通道
}
```

**SSE端点：**
```go
http.HandleFunc("/api/logs", s.handleLogs)
```

**日志发送函数：**
```go
func (s *Server) sendLog(format string, args ...interface{}) {
    msg := fmt.Sprintf(format, args...)
    s.logChannel <- msg
}
```

### 2. 修改审核函数

**本地模式（handleReview）：**
- ✅ 审核在后台goroutine中执行
- ✅ 使用 `sendLog` 发送实时日志
- ✅ 立即返回响应，不阻塞

**日志输出示例：**
```go
s.sendLog("开始审核 %d 个文件...", len(filesToReview))
s.sendLog("[%d/%d] 正在审核: %s", i+1, len(filesToReview), change.Path)
s.sendLog("  ✅ 审核完成")
s.sendLog("✅ 报告已生成: %s", absPath)
s.sendLog("所有文件审核完成！")
```

## 前端实现（需要添加）

### 1. 修改 index.html

在 `<script>` 标签中添加SSE连接：

```javascript
// 连接SSE日志流
let eventSource = null;

function connectLogs() {
    if (eventSource) {
        eventSource.close();
    }
    
    eventSource = new EventSource('/api/logs');
    
    eventSource.onmessage = function(event) {
        log(event.data);
    };
    
    eventSource.onerror = function(error) {
        console.error('SSE Error:', error);
        // 自动重连
        setTimeout(connectLogs, 5000);
    };
}

// 页面加载时连接
window.onload = () => {
    loadWorkDir();
    loadConfig();
    connectLogs(); // 连接SSE
};
```

### 2. 修改 startReview 函数

```javascript
async function startReview() {
    if (selectedIndices.size === 0) {
        alert('请至少选择一个文件');
        return;
    }

    const workDir = document.getElementById('workDir').value;
    saveWorkDir(workDir);
    
    const reviewBtn = document.getElementById('reviewBtn');
    reviewBtn.disabled = true;
    reviewBtn.innerHTML = '<span class="loading"></span> 审核中...';
    
    // 清空日志
    document.getElementById('logArea').textContent = '';
    
    try {
        const response = await fetch('/api/review', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ 
                work_dir: workDir,
                indices: Array.from(selectedIndices)
            })
        });
        
        const data = await response.json();
        if (data.error) {
            log('❌ ' + data.error);
            alert('审核失败: ' + data.error);
        } else {
            log('✅ ' + data.message);
            // 日志会通过SSE实时显示
        }
    } catch (error) {
        log('❌ 请求失败: ' + error.message);
    } finally {
        reviewBtn.disabled = false;
        reviewBtn.textContent = '开始审核';
    }
}
```

### 3. 同样修改 online.html

添加相同的SSE连接代码。

## 测试步骤

### 1. 启动服务器
```bash
svn-reviewer.exe
```

### 2. 打开浏览器
```
http://localhost:8080
```

### 3. 执行审核
1. 加载配置
2. 扫描变更
3. 选择文件
4. 点击"开始审核"

### 4. 观察日志
日志区域应该实时显示：
```
开始审核 2 个文件...
[1/2] 正在审核: src/Main.java
  ✅ 审核完成
[2/2] 正在审核: src/Utils.java
  ✅ 审核完成
正在生成HTML报告...
✅ 报告已生成: F:\path\to\report.html
所有文件审核完成！
```

## 优势

### 1. 实时反馈
- ✅ 用户可以看到当前正在审核哪个文件
- ✅ 可以看到每个文件的审核结果
- ✅ 类似控制台的体验

### 2. 不阻塞界面
- ✅ 审核在后台进行
- ✅ 用户可以继续操作界面
- ✅ 响应更快

### 3. 更好的用户体验
- ✅ 清晰的进度显示
- ✅ 详细的状态信息
- ✅ 错误信息实时显示

## 在线模式实现（待完成）

需要同样修改 `handleOnlineReview` 函数：

```go
func (s *Server) handleOnlineReview(w http.ResponseWriter, r *http.Request) {
    // ... 验证和准备 ...
    
    // 在后台执行审核
    go func() {
        s.sendLog("开始审核 %d 个文件...", len(filesToReview))
        
        for i, file := range filesToReview {
            s.sendLog("[%d/%d] 正在审核: %s (r%d)", i+1, len(filesToReview), file.Path, file.Revision)
            
            // 删除的文件跳过
            if file.Status == "D" {
                s.sendLog("  ℹ️  删除的文件，跳过审核")
                continue
            }
            
            // ... 审核逻辑 ...
            
            s.sendLog("  ✅ 审核完成")
        }
        
        s.sendLog("正在生成HTML报告...")
        // ... 生成报告 ...
        s.sendLog("✅ 报告已生成: %s", absPath)
        s.sendLog("所有文件审核完成！")
    }()
    
    // 立即返回
    respondJSON(w, map[string]interface{}{
        "success": true,
        "message": "审核已开始，请查看日志",
    }, http.StatusOK)
}
```

## 注意事项

### 1. SSE连接管理
- 页面关闭时自动断开
- 连接断开时自动重连
- 避免内存泄漏

### 2. 日志通道容量
```go
logChannel: make(chan string, 100)
```
- 容量100条消息
- 满了会丢弃消息
- 可以根据需要调整

### 3. 并发安全
- 使用channel传递消息
- Go的channel是并发安全的
- 不需要额外的锁

## 完整的前端代码示例

```html
<script>
let eventSource = null;

function connectLogs() {
    if (eventSource) {
        eventSource.close();
    }
    
    eventSource = new EventSource('/api/logs');
    
    eventSource.onmessage = function(event) {
        log(event.data);
    };
    
    eventSource.onerror = function(error) {
        console.error('SSE Error:', error);
        setTimeout(connectLogs, 5000);
    };
}

function log(message) {
    const logArea = document.getElementById('logArea');
    const timestamp = new Date().toLocaleTimeString();
    logArea.textContent += `[${timestamp}] ${message}\n`;
    logArea.scrollTop = logArea.scrollHeight;
}

window.onload = () => {
    loadWorkDir();
    loadConfig();
    connectLogs();
};

// 页面卸载时关闭连接
window.onbeforeunload = () => {
    if (eventSource) {
        eventSource.close();
    }
};
</script>
```

## 总结

后端SSE实现已完成：
- ✅ SSE端点 `/api/logs`
- ✅ 日志发送函数 `sendLog`
- ✅ 本地模式审核已支持实时日志
- ⏳ 在线模式审核待添加
- ⏳ 前端SSE连接待添加

下一步：
1. 修改前端HTML添加SSE连接
2. 修改在线模式审核函数
3. 测试实时日志功能

---

**版本**: 1.4.0  
**更新日期**: 2024-01-15  
**状态**: ✅ 后端完成，前端待实现
