@echo off
echo 正在构建 SVN 代码审核工具...
go mod download
go build -o svn-reviewer.exe
if %errorlevel% equ 0 (
    echo.
    echo 构建成功！可执行文件: svn-reviewer.exe
    echo.
    echo 使用方法:
    echo   svn-reviewer review          - 审核当前目录
    echo   svn-reviewer review -d PATH  - 审核指定目录
    echo   svn-reviewer review -i       - 交互式选择文件
    echo   svn-reviewer review --help   - 查看帮助
) else (
    echo.
    echo 构建失败，请检查错误信息
)
pause
