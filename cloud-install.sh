#!/bin/bash
# ============================================================
#  VPN 管理面板 云端一键安装脚本
#  一条命令安装 (无需先克隆仓库):
#    bash <(curl -fsSL https://raw.githubusercontent.com/ccmaoxiong/vpn-panel/main/cloud-install.sh)
#
#  用法:
#    curl ... | bash                 # 安装并启动
#    curl ... | bash -s -- update    # 更新到最新版本
#    curl ... | bash -s -- uninstall # 卸载 (保留数据)
#    curl ... | bash -s -- status    # 查看状态
#  环境变量:
#    PORT=8080 curl ... | bash                 # 自定义端口
#    INSTALL_METHOD=source curl ... | bash     # 强制源码编译安装
# ============================================================
set -e

REPO="ccmaoxiong/vpn-panel"
GITHUB="https://github.com/${REPO}"
RAW="https://raw.githubusercontent.com/${REPO}/main"
API="https://api.github.com/repos/${REPO}"

APP="vpn-panel"
GO_VERSION="${GO_VERSION:-1.24.4}"
PORT="${PORT:-5000}"
HOST="${HOST:-0.0.0.0}"
METHOD="${INSTALL_METHOD:-auto}"   # auto | binary | source

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

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) echo "" ;;
    esac
}

latest_tag() {
    curl -fsSL --connect-timeout 10 "${API}/releases/latest" 2>/dev/null \
        | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1
}

# ---------- 方式一: 下载预编译二进制 ----------
download_binary() {
    local arch tag url
    arch=$(detect_arch)
    [ -z "$arch" ] && return 1
    tag=$(latest_tag)
    [ -z "$tag" ] && return 1
    url="${GITHUB}/releases/download/${tag}/${APP}-linux-${arch}"
    info "下载预编译版本 ${tag} (${arch}) ..."
    mkdir -p "$INSTALL_DIR"
    curl -fL --connect-timeout 15 --retry 2 -o "${INSTALL_DIR}/${APP}" "$url"
    chmod +x "${INSTALL_DIR}/${APP}"
}

# ---------- 方式二: 源码编译 ----------
install_go() {
    if command -v go >/dev/null 2>&1; then
        local minor
        minor=$(go version | sed -n 's/.*go1\.\([0-9]*\).*/\1/p')
        if [ "${minor:-0}" -ge 22 ]; then
            info "检测到 Go $(go version | awk '{print $3}')，跳过安装"
            return 0
        fi
    fi
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64) TAR="go${GO_VERSION}.linux-amd64.tar.gz" ;;
        aarch64|arm64) TAR="go${GO_VERSION}.linux-arm64.tar.gz" ;;
        *) err "不支持的架构: $arch"; exit 1 ;;
    esac
    info "下载 Go ${GO_VERSION} ..."
    local tmp="/tmp/${TAR}"
    local url="https://go.dev/dl/${TAR}"
    if ! curl -fL --connect-timeout 15 --retry 2 -o "$tmp" "$url" 2>/dev/null; then
        info "官方源失败，尝试国内镜像 golang.google.cn ..."
        url="https://golang.google.cn/dl/${TAR}"
        curl -fL --retry 2 -o "$tmp" "$url"
    fi
    local prefix
    if [ "$(id -u)" -eq 0 ]; then prefix="/usr/local"; else prefix="$HOME/.local"; fi
    mkdir -p "$prefix"
    rm -rf "$prefix/go"
    tar -C "$prefix" -xzf "$tmp"
    rm -f "$tmp"
    export PATH="$prefix/go/bin:$PATH"
    grep -q "$prefix/go/bin" "$HOME/.profile" 2>/dev/null \
        || echo "export PATH=\"$prefix/go/bin:\$PATH\"" >> "$HOME/.profile"
    info "Go 安装完成: $(go version)"
}

build_from_source() {
    info "源码编译安装 ..."
    install_go
    local tmp
    tmp=$(mktemp -d)
    git clone --depth 1 "${GITHUB}.git" "$tmp"
    (cd "$tmp" && go build -trimpath -ldflags "-s -w" -o "${INSTALL_DIR}/${APP}" .)
    rm -rf "$tmp"
    info "编译完成"
}

# ---------- 安装主流程 ----------
do_install() {
    info "=== VPN 管理面板 云端安装开始 ==="
    local method="$METHOD"
    if [ "$method" = "auto" ]; then
        if download_binary; then
            method="binary"
        else
            warn "二进制下载失败，回退到源码编译"
            build_from_source
            method="source"
        fi
    elif [ "$method" = "binary" ]; then
        download_binary
    else
        build_from_source
    fi
    mkdir -p "$DATA_DIR"
    echo "$method" > "${DATA_DIR}/.install_method"

    if [ -n "$SERVICE_FILE" ]; then
        mkdir -p "$DATA_DIR"
        cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=VPN Admin Panel
After=network.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/${APP}
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
    else
        warn "非 root 运行，跳过 systemd 服务"
        info "手动启动: ${INSTALL_DIR}/${APP}"
    fi

    if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
        warn "检测到 ufw 防火墙，如需外网访问请执行: ufw allow ${PORT}/tcp"
    elif command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state 2>/dev/null | grep -q running; then
        warn "检测到 firewalld 防火墙，如需外网访问请执行: firewall-cmd --add-port=${PORT}/tcp --permanent && firewall-cmd --reload"
    fi

    echo ""
    info "=== 安装完成 ==="
    info "安装方式: ${method}"
    info "访问地址: http://$(hostname -I 2>/dev/null | awk '{print $1}'):${PORT}"
    info "默认账号: admin / admin123 (登录后请立即修改密码)"
    info "数据文件: ${DATA_DIR}/data.json"
    if [ -n "$SERVICE_FILE" ]; then
        info "服务管理: systemctl start|stop|restart|status vpn-panel"
        info "查看日志: journalctl -u vpn-panel -f"
    fi
}

do_update() {
    info "=== 更新 ${APP} ==="
    local method
    method=$(cat "${DATA_DIR}/.install_method" 2>/dev/null || echo "auto")
    if [ "$method" = "binary" ] && [ "$METHOD" != "source" ] && download_binary; then
        :
    else
        warn "二进制更新失败，回退源码编译"
        build_from_source
        mkdir -p "$DATA_DIR"
        echo "source" > "${DATA_DIR}/.install_method"
    fi
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
    warn "已卸载。数据目录 ${DATA_DIR} 已保留，如需删除: rm -rf ${DATA_DIR}"
}

do_status() {
    if [ -n "$SERVICE_FILE" ] && systemctl is-active --quiet vpn-panel; then
        systemctl status vpn-panel --no-pager
    elif pgrep -f "$APP" >/dev/null; then
        info "进程运行中 (PID: $(pgrep -f "$APP" | head -1))"
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
