@echo off
chcp 65001 >nul
cd /d "%~dp0"
echo ============================================
echo   VPN 管理面板 (Go + HTML + JavaScript)
echo ============================================
where go >nul 2>nul
if errorlevel 1 (
    echo [错误] 未检测到 Go 环境，请先安装: https://go.dev/dl/
    pause
    exit /b 1
)
echo [1/2] 编译中...
go build -o vpn-panel.exe .
if errorlevel 1 (
    echo [错误] 编译失败，请检查上方错误信息
    pause
    exit /b 1
)
echo [2/2] 启动面板: http://127.0.0.1:5000
vpn-panel.exe
pause
