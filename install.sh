#!/bin/bash
# ============================================================
#  VPN 管理面板 一键安装脚本 (Linux)
#  用法:
#    ./install.sh            # 安装并启动 (systemd 服务)
#    ./install.sh update     # 更新 (重新编译并重启)
#    ./install.sh uninstall  # 卸载 (默认保留数据)
#    ./install.sh status     # 查看状态
#  环境变量: PORT=端口(默认5000) HOST=监听地址(默认0.0.0.0)
# ============================================================
set -e

APP_NAME="vpn-panel"
GO_VERSION="${GO_VERSION:-1.24.4}"
PORT="${PORT:-5000}"
HOST="${HOST:-0.0.0.0}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

if [ "$(id -u)" -eq 0 ]; then
    INSTALL_DIR="/usr/local/vpn-panel"
    DATA_DIR="/var/lib/vpn-panel"
    SERVICE_FILE="/etc/systemd/system/vpn-panel.service"
else
    INSTALL_DIR="$HOME/.local/vpn-panel"
    DATA_DIR="$HOME/.local/share/vpn-panel"
    SERVICE_FILE=""
fi

info()  { echo -e "\033[32m[信息]\033[0m $1"; }
warn()  { echo -e "\033[33m[警告]\033[0m $1"; }
err()   { echo -e "\033[31m[错误]\033[0m $1"; }

# ---------- 安装 Go ----------
install_go() {
    if command -v go >/dev/null 2>&1; then
        local minor
        minor=$(go version | sed -n 's/.*go1\.\([0-9]*\).*/\1/p')
        if [ "${minor:-0}" -ge 22 ]; then
            info "检测到 Go $(go version | awk '{print $3}') (需 >= 1.22)，跳过安装"
            return 0
        fi
        warn "Go 版本过低，重新安装"
    fi

    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64)  TAR="go${GO_VERSION}.linux-amd64.tar.gz" ;;
        aarch64|arm64) TAR="go${GO_VERSION}.linux-arm64.tar.gz" ;;
        *) err "不支持的架构: $arch"; exit 1 ;;
    esac

    info "下载 Go ${GO_VERSION} ($arch) ..."
    local tmp="/tmp/${TAR}"
    local url="https://go.dev/dl/${TAR}"
    if ! curl -fL --connect-timeout 15 --retry 2 -o "$tmp" "$url" 2>/dev/null; then
        info "官方源失败，尝试国内镜像 golang.google.cn ..."
        url="https://golang.google.cn/dl/${TAR}"
        curl -fL --retry 2 -o "$tmp" "$url"
    fi

    local prefix
    if [ "$(id -u)" -eq 0 ]; then
        prefix="/usr/local"
    else
        prefix="$HOME/.local"
    fi
    mkdir -p "$prefix"
    rm -rf "$prefix/go"
    tar -C "$prefix" -xzf "$tmp"
    rm -f "$tmp"

    export PATH="$prefix/go/bin:$PATH"
    if ! grep -q "$prefix/go/bin" "$HOME/.profile" 2>/dev/null; then
        echo "export PATH=\"$prefix/go/bin:\$PATH\"" >> "$HOME/.profile"
    fi
    info "Go 安装完成: $(go version)"
}

# ---------- 编译 ----------
build() {
    info "编译 ${APP_NAME} ..."
    mkdir -p "$INSTALL_DIR"
    go build -trimpath -ldflags "-s -w" -o "${INSTALL_DIR}/${APP_NAME}" .
    info "编译完成: ${INSTALL_DIR}/${APP_NAME}"
}

# ---------- systemd 服务 ----------
install_service() {
    if [ -z "$SERVICE_FILE" ]; then
        warn "非 root 运行，跳过 systemd 服务安装"
        info "手动启动: ${INSTALL_DIR}/${APP_NAME}"
        return 0
    fi
    mkdir -p "$DATA_DIR"
    cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=VPN Admin Panel
After=network.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/${APP_NAME}
Environment=PORT=${PORT}
Environment=HOST=${HOST}
Environment=DATA_FILE=${DATA_DIR}/data.json
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable --now vpn-panel >/dev/null 2>&1
    info "systemd 服务已注册并启动"
}

firewall_hint() {
    if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
        warn "检测到 ufw 防火墙，如需外网访问请执行: ufw allow ${PORT}/tcp"
    elif command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state 2>/dev/null | grep -q running; then
        warn "检测到 firewalld 防火墙，如需外网访问请执行: firewall-cmd --add-port=${PORT}/tcp --permanent && firewall-cmd --reload"
    fi
}

# ---------- 主流程 ----------
do_install() {
    info "=== VPN 管理面板 安装开始 ==="
    install_go
    build
    install_service
    firewall_hint
    echo ""
    info "=== 安装完成 ==="
    info "访问地址: http://$(hostname -I 2>/dev/null | awk '{print $1}'):${PORT}"
    info "默认账号: admin / admin123 (登录后请立即修改密码)"
    info "数据文件: ${DATA_DIR}/data.json"
    if [ -n "$SERVICE_FILE" ]; then
        info "服务管理: systemctl start|stop|restart|status vpn-panel"
        info "查看日志: journalctl -u vpn-panel -f"
    fi
}

do_update() {
    info "=== 更新 ${APP_NAME} ==="
    install_go
    build
    if [ -n "$SERVICE_FILE" ]; then
        systemctl restart vpn-panel
        info "服务已重启"
    else
        warn "非 root 运行，请手动重启进程"
    fi
    info "更新完成"
}

do_uninstall() {
    if [ -n "$SERVICE_FILE" ]; then
        systemctl disable --now vpn-panel >/dev/null 2>&1 || true
        rm -f "$SERVICE_FILE"
        systemctl daemon-reload
    fi
    rm -rf "$INSTALL_DIR"
    warn "已卸载。数据目录 ${DATA_DIR} 已保留，如需删除请执行: rm -rf ${DATA_DIR}"
}

do_status() {
    if [ -n "$SERVICE_FILE" ] && systemctl is-active --quiet vpn-panel; then
        systemctl status vpn-panel --no-pager
    elif pgrep -f "$APP_NAME" >/dev/null; then
        info "进程运行中 (PID: $(pgrep -f "$APP_NAME" | head -1))"
    else
        warn "面板未运行"
    fi
}

case "${1:-install}" in
    install)   do_install ;;
    update)    do_update ;;
    uninstall) do_uninstall ;;
    status)    do_status ;;
    *) echo "用法: $0 [install|update|uninstall|status]"; exit 1 ;;
esac
