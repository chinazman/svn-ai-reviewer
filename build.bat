@echo off
echo 正在构建 SVN 代码审核工具...
go mod tidy
go build -o svn-reviewer.exe
if %errorlevel% equ 0 (
    echo.
    echo 构建成功！可执行文件: svn-reviewer.exe
    echo.
    echo 使用方法:
    echo   双击 svn-reviewer.exe        - 启动 Web GUI 界面 (推荐)
    echo   svn-reviewer review          - CLI 模式：审核当前目录
    echo   svn-reviewer review -d PATH  - CLI 模式：审核指定目录
    echo   svn-reviewer review -i       - CLI 模式：交互式选择文件
    echo   svn-reviewer review --help   - 查看帮助
) else (
    echo.
    echo 构建失败，请检查错误信息
)
pause
