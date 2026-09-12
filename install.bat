@echo off
chcp 65001 >nul
cd /d "%~dp0"
setlocal enabledelayedexpansion

set GO_VERSION=1.24.4
set GO_ZIP=go%GO_VERSION%.windows-amd64.zip
set GO_ROOT=%USERPROFILE%\go-toolchain\go

echo ============================================
echo   VPN 管理面板 一键安装 (Windows)
echo ============================================
echo.

:: ---------- 1. 检查/安装 Go ----------
set GO_FOUND=0
where go >nul 2>nul
if %errorlevel%==0 (
    for /f "tokens=3" %%v in ('go version') do echo [信息] 已检测到 Go %%v
    set GO_FOUND=1
    goto :build
)
if exist "%GO_ROOT%\bin\go.exe" (
    set "PATH=%GO_ROOT%\bin;%PATH%"
    echo [信息] 使用已安装的 Go 工具链: %GO_ROOT%
    goto :build
)

echo [信息] 未检测到 Go，开始下载 Go %GO_VERSION% (约 70MB) ...
set "GO_DOWNLOAD=%TEMP%\%GO_ZIP%"
powershell -NoProfile -Command "[Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12; Invoke-WebRequest -Uri 'https://go.dev/dl/%GO_ZIP%' -OutFile '%GO_DOWNLOAD%' -UseBasicParsing"
if not exist "%GO_DOWNLOAD%" (
    echo [信息] 官方源失败，尝试国内镜像 ...
    powershell -NoProfile -Command "[Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12; Invoke-WebRequest -Uri 'https://golang.google.cn/dl/%GO_ZIP%' -OutFile '%GO_DOWNLOAD%' -UseBasicParsing"
)
if not exist "%GO_DOWNLOAD%" (
    echo [错误] Go 下载失败，请手动安装: https://go.dev/dl/
    pause
    exit /b 1
)
echo [信息] 解压 Go 到 %USERPROFILE%\go-toolchain ...
powershell -NoProfile -Command "Expand-Archive -Path '%GO_DOWNLOAD%' -DestinationPath '%USERPROFILE%\go-toolchain' -Force"
if not exist "%GO_ROOT%\bin\go.exe" (
    echo [错误] 解压失败
    pause
    exit /b 1
)
set "PATH=%GO_ROOT%\bin;%PATH%"
echo [信息] Go 安装完成: 
"%GO_ROOT%\bin\go.exe" version

:build
:: ---------- 2. 编译 ----------
echo [信息] 编译 vpn-panel.exe ...
go build -trimpath -ldflags "-s -w" -o vpn-panel.exe .
if errorlevel 1 (
    echo [错误] 编译失败，请检查上方错误信息
    pause
    exit /b 1
)
echo [信息] 编译完成: %~dp0vpn-panel.exe

:: ---------- 3. 开机自启 ----------
echo.
set /p AUTO="是否注册开机自启动任务? (Y/N): "
if /i "%AUTO%"=="Y" (
    schtasks /create /tn "VPNPanel" /tr "\"%~dp0vpn-panel.exe\"" /sc onstart /rl highest /f >nul 2>nul
    if %errorlevel%==0 (
        echo [信息] 已注册开机自启任务 "VPNPanel" (schtasks /delete /tn VPNPanel 可删除)
    ) else (
        echo [警告] 开机自启注册失败 (需要管理员权限)
    )
)

:: ---------- 4. 启动 ----------
echo.
echo [信息] 启动面板: http://127.0.0.1:5000
echo [信息] 默认账号: admin / admin123
echo [信息] 数据文件: %~dp0data.json
start "" vpn-panel.exe
echo.
pause
