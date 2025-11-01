# HTTP 报告服务测试说明

## 功能说明

服务器现在将 `reports` 目录作为静态文件服务，审核完成后会自动在浏览器新标签页中打开报告�?HTTP 地址�?

## 测试步骤

### 1. 启动服务�?

```bash
svn-ai-reviewer.exe gui
```

服务器启动后会显示：
```
🚀 SVN 代码审核工具已启�?
📱 本地模式: http://localhost:8080
📱 在线模式: http://localhost:8080/online
📊 报告目录: http://localhost:8080/reports/
�?Ctrl+C 停止服务�?
```

### 2. 测试本地模式

1. 访问 http://localhost:8080
2. 加载配置文件（默�?config.yaml�?
3. 扫描 SVN 变更
4. 选择文件并开始审�?
5. 审核完成后，报告会自动在新标签页打开
6. 报告地址格式：`http://localhost:8080/reports/report-20250101-120000.html`

### 3. 测试在线模式

1. 访问 http://localhost:8080/online
2. 加载配置文件
3. 连接 SVN 服务�?
4. 搜索提交记录
5. 选择版本和文�?
6. 开始审�?
7. 审核完成后，报告会自动在新标签页打开

### 4. 手动访问报告

可以直接访问报告目录查看所有历史报告：
- 访问 http://localhost:8080/reports/
- 浏览器会显示目录列表（如果服务器支持�?
- 或者直接访问具体的报告文件

## 验证要点

1. �?报告能通过 HTTP 地址访问
2. �?审核完成后自动在新标签页打开报告
3. �?日志中显�?`REPORT_URL:http://localhost:8080/reports/...`
4. �?报告可以正常显示，样式和功能正常
5. �?可以分享报告 URL 给其他人访问（如果服务器可访问）

## 注意事项

1. 确保 `reports` 目录存在（首次运行会自动创建�?
2. 报告文件名格式：`report-YYYYMMDD-HHMMSS.html`
3. 如果需要从外部访问，需要修改服务器监听地址（从 `localhost:8080` 改为 `0.0.0.0:8080`�?
4. 报告文件会一直保留，可以手动清理旧报�?

## 与之前的区别

### 之前（file:// 协议�?
- 报告地址：`file:///F:/path/to/reports/report.html`
- 只能在本机访�?
- 受浏览器本地文件访问限制
- 无法分享

### 现在（HTTP 协议�?
- 报告地址：`http://localhost:8080/reports/report.html`
- 可以在网络内任何设备访问
- 无浏览器限制
- 可以分享 URL
- 更符�?Web 应用架构
