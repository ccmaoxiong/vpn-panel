# VPN 管理面板

类 3x-ui 风格的 Web VPN 管理后台，使用 **Go + HTML + JavaScript** 编写，纯 Go 标准库实现（零第三方依赖），数据存储为本地 JSON 文件，编译后为单文件部署。

## 一键安装

### ☁️ 云服务器一条命令安装 (推荐)

直接下载 GitHub Releases 预编译二进制,无需安装 Go:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/ccmaoxiong/vpn-panel/main/cloud-install.sh)
```

```bash
bash <(curl -fsSL .../cloud-install.sh) update     # 更新到最新版
bash <(curl -fsSL .../cloud-install.sh) uninstall  # 卸载 (保留数据)
bash <(curl -fsSL .../cloud-install.sh) status     # 查看状态
PORT=8080 bash <(curl -fsSL .../cloud-install.sh)  # 自定义端口
```

- 自动检测架构 (amd64/arm64),优先下载 Releases 二进制,失败自动回退源码编译
- Releases 由 GitHub Actions 自动构建: 推送 `v*` 标签触发,或在 Actions 页面手动运行 `release` 工作流

### Linux (从源码克隆安装)

```bash
git clone https://github.com/ccmaoxiong/vpn-panel.git
cd vpn-panel
sudo ./install.sh
```

脚本自动完成: 检测/安装 Go → 编译 → 注册 systemd 服务 → 启动。

```bash
./install.sh update     # 更新 (重新编译并重启)
./install.sh uninstall  # 卸载 (保留数据)
./install.sh status     # 查看状态
PORT=8080 sudo ./install.sh   # 自定义端口
```

访问 `http://服务器IP:5000`，默认账号 `admin / admin123`（登录后请立即修改密码）。

### Windows

双击 `install.bat`，脚本自动: 下载安装 Go（如缺失）→ 编译 → 可选注册开机自启 → 启动面板。

## 手动安装

需要 Go 1.22+:

```bash
go build -o vpn-panel .
./vpn-panel
```

Windows 直接 `go build` 或双击 `start.bat`。

## 环境变量

| 变量 | 默认 | 说明 |
|---|---|---|
| `PORT` | 5000 | 监听端口 |
| `HOST` | 127.0.0.1 | 监听地址 (Linux 安装脚本默认 0.0.0.0) |
| `DATA_FILE` | data.json | 数据文件路径 |

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go 标准库 (net/http, html/template, encoding/json) |
| 前端 | 原生 JavaScript + HTML + CSS（深色 3x-ui 风格界面） |
| 存储 | JSON 文件 (data.json)，内存读写 + 互斥锁保护 |
| 认证 | Cookie 会话 + SHA-256 加盐密码哈希 |
| 资源 | `go:embed` 内嵌模板与静态资源，单文件部署 |

## 功能

| 模块 | 说明 |
|---|---|
| 仪表盘 | 用户/节点/流量概览、最近操作日志 |
| 用户管理 | 增删改查、启用禁用、协议 (VLESS/VMess/Trojan/Shadowsocks)、SS 加密方式、VMess 加密 (scy)、流量配额、有效期、设备限制、重置流量、重置 UUID |
| 套餐管理 | 流量/时长/价格/设备限制套餐，绑定用户自动填充配额 |
| 节点管理 | 地址/端口/传输 (TCP/WS/HTTPUpgrade/gRPC/mKCP)/安全 (TLS/Reality/None)/SNI/Flow/Reality 参数 (pbk/sid/spx/fp) |
| 订阅管理 | 每用户独立 Base64 订阅、复制/下载、全部导出 |
| 流量统计 | 用户流量排行、使用率、流量明细记录 |
| 操作日志 | 管理员操作审计、IP 记录、一键清空 |
| 系统设置 | 站点名称、订阅域名、流量重置周期、修改密码、SSL 证书 (HTTPS) |

## 其他

- 配置查看弹窗内置二维码（纯 JS 生成，无外部依赖）
- 订阅链接格式: `https://{订阅域名}/sub/{token}`（在系统设置中配置 sub_domain）
- 模拟流量写入 API `POST /api/traffic` 用于演示/对接 Xray 流量上报
- 测试: `go test ./...`

## License

[MIT](LICENSE)
