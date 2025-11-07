@echo off
chcp 65001 >nul
echo ========================================
echo   SVN 代码审核工具 - 源代码模式测试
echo ========================================
echo.

echo [1/3] 检查程序是否存在...
if not exist "svn-ai-reviewer.exe" (
    echo ❌ 未找到 svn-ai-reviewer.exe
    echo 请先运行 build.bat 编译程序
    pause
    exit /b 1
)
echo ✅ 程序文件存在

echo.
echo [2/3] 检查配置文件...
if not exist "config.yaml" (
    echo ❌ 未找到 config.yaml
    echo 请先创建配置文件
    pause
    exit /b 1
)
echo ✅ 配置文件存在

echo.
echo [3/3] 启动服务...
echo.
echo 服务启动后，请在浏览器中访问：
echo   http://localhost:8080/source
echo.
echo 测试步骤：
echo   1. 选择配置文件（默认 config.yaml）
echo   2. 输入路径：.
echo   3. 输入过滤：*.go
echo   4. 点击"扫描文件"
echo   5. 勾选1-2个文件
echo   6. 点击"开始审核"
echo.
echo 按 Ctrl+C 停止服务器
echo.
echo ========================================
echo.

svn-ai-reviewer.exe

pause
