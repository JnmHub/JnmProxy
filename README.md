# JnmProxy

JnmProxy 是一个带 Web 管理后台的本地代理池系统。你可以把多个机场订阅导入进来，系统会解析节点、定时刷新、按分组或凭证选择节点，并对外提供 HTTP / SOCKS5 代理入口。

它适合这些场景：

- 你有多个订阅链接，希望统一管理所有节点。
- 你希望不同账号使用不同节点范围，比如“全部节点随机”“某个分组随机”“固定某个节点”。
- 你希望代理请求失败时自动换节点，减少手动切换。
- 你希望有一个本地后台查看节点、分组、订阅用量、请求失败原因和流量统计。

> JnmProxy 不提供节点、不售卖代理服务，只负责管理你自己添加的订阅和节点。请遵守当地法律法规以及订阅服务商的使用规则。

## 主要能力

### 订阅管理

- 支持添加多个订阅链接。
- 支持手动刷新和定时刷新订阅。
- 支持读取订阅用量、总量和到期时间。
- 支持 Clash YAML、Base64 订阅、逐行 URI 订阅。
- 刷新订阅时会自动识别重复节点，避免节点数量越刷越多。
- 不保存机场规则、分流规则和策略组，只保存代理节点。

### 节点管理

- 所有订阅节点统一入库，并保留来源订阅标识。
- 支持按订阅、协议、地区、健康状态、sing-box 状态搜索和筛选。
- 支持启用 / 禁用节点。
- 支持健康检查，死亡节点不会进入运行时代理池。
- 支持查看节点 sing-box 适配状态和错误原因。
- 支持分页加载，节点很多时后台不会一次拉完整表。

### 分组管理

- 支持手动创建分组。
- 支持单个节点加入多个分组。
- 支持批量把节点加入或移出分组。
- 支持关键词自动分组，例如 `香港|日本|美国`。
- 关键词匹配后会自动创建对应分组，并把匹配节点加入分组。

### 凭证与调度

JnmProxy 的代理入口必须带用户名和密码，不带凭证会被拒绝。

凭证支持三种绑定方式：

- `全部节点`：请求时从所有可用节点里选择。
- `指定分组`：请求时只从绑定分组内选择。
- `固定节点`：请求永远使用指定节点。

节点选择支持：

- 随机选择节点。
- 固定使用某个节点。
- 小范围内尽量减少连续重复选择。
- 请求失败时在同一次请求内自动换下一个候选节点。
- 节点连续失败后进入短暂熔断，恢复时间到后再重新参与调度。

### 代理入口

默认提供两个本地代理入口：

- HTTP 代理：`127.0.0.1:1081`
- SOCKS5 代理：`127.0.0.1:1080`

使用示例：

```bash
curl -x http://用户名:密码@127.0.0.1:1081 https://httpbin.org/ip
```

```bash
curl --proxy socks5h://用户名:密码@127.0.0.1:1080 https://httpbin.org/ip
```

### 协议支持

JnmProxy 不使用 Clash 内核。复杂协议能力通过 Go 依赖内嵌 `github.com/sagernet/sing-box`，不要求你在系统里单独安装 sing-box 命令。

当前主要支持：

- HTTP / HTTPS
- SOCKS / SOCKS5 / SOCKS5H
- Shadowsocks / SS
- VMess
- VLESS
- Trojan
- Hysteria2 / HY2，需要使用带 `with_quic` 的构建
- TUIC，需要使用带 `with_quic` 的构建

REALITY 或带 `client-fingerprint` 的 TLS 节点需要使用带 `with_utls` 的构建。

## 快速开始

### 方式一：下载发行版

打开 Release 页面下载对应系统的压缩包：

```text
https://github.com/JnmHub/JnmProxy/releases
```

常见文件名示例：

- macOS Apple Silicon：`jnmproxy-版本-darwin-arm64.zip`
- macOS Intel：`jnmproxy-版本-darwin-amd64.zip`
- Linux x64：`jnmproxy-版本-linux-amd64.zip`
- Linux ARM64：`jnmproxy-版本-linux-arm64.zip`
- Windows x64：`jnmproxy-版本-windows-amd64.zip`

解压后进入目录，复制配置文件：

```bash
cp config.example.yaml config.yaml
```

启动：

```bash
./jnmproxy -config config.yaml
```

Windows PowerShell：

```powershell
.\jnmproxy.exe -config config.yaml
```

macOS 如果提示无法打开，可以在解压目录执行：

```bash
xattr -dr com.apple.quarantine ./jnmproxy
```

启动后访问管理后台：

```text
http://127.0.0.1:8080/
```

### 方式二：源码启动

需要安装：

- Go 1.24.7 或更高版本
- Node.js 22 或更高版本，仅前端开发或重新构建前端时需要

后端启动：

```bash
go run ./cmd/jnmproxy
```

如果你需要 Hysteria2 / TUIC / REALITY 等能力，建议使用：

```bash
go run -tags "with_quic with_utls" ./cmd/jnmproxy
```

默认会自动创建 SQLite 数据库：

```text
./data/jnmproxy.db
```

## 第一次使用流程

### 1. 打开后台

浏览器访问：

```text
http://127.0.0.1:8080/
```

### 2. 添加订阅

进入“订阅”页面，填写：

- 名称：自己能看懂即可。
- 订阅链接：你的机场订阅地址。
- User-Agent：一般保持默认 `clash.meta` 即可。
- 刷新间隔：例如 `3600` 秒。

保存后点击刷新订阅，系统会把解析出来的节点放入节点池。

如果某些订阅要求 Clash 客户端标识，可以把 User-Agent 改为：

```text
clash/1.18.0
```

### 3. 创建代理访问凭证

进入“凭证”页面，创建用户名和密码。

你可以选择：

- 不绑定分组：使用全部节点。
- 绑定分组：只使用这个分组里的节点。
- 绑定固定节点：永远使用这个节点。

### 4. 复制代理命令测试

在“凭证”页面可以复制 HTTP 或 SOCKS5 命令，也可以手写：

```bash
curl -x http://用户名:密码@127.0.0.1:1081 https://httpbin.org/ip
```

```bash
curl --proxy socks5h://用户名:密码@127.0.0.1:1080 https://httpbin.org/ip
```

如果返回的 IP 是代理出口 IP，说明代理池已经可以使用。

## 配置文件

默认配置文件是 `config.yaml`，可以从 `config.example.yaml` 复制。

常用配置：

```yaml
server:
  api_addr: "127.0.0.1:8080"

proxy:
  http_addr: "127.0.0.1:1081"
  socks_addr: "127.0.0.1:1080"

database:
  path: "./data/jnmproxy.db"

subscription:
  default_user_agent: "clash.meta"
  default_refresh_interval_seconds: 3600
  request_timeout_seconds: 20

runtime:
  max_attempts_per_request: 3
  failure_threshold: 3
  circuit_break_seconds: 60
  record_failed_requests: true

scheduler:
  subscription_tick_seconds: 30
  health_check_interval_seconds: 300
  health_check_target: "www.gstatic.com:443"

sing_box:
  enabled: true
  mode: "auto"
  prefer_native_http_socks: true
  enable_udp: false

admin:
  token: ""
```

说明：

- `server.api_addr`：管理后台和 API 监听地址。
- `proxy.http_addr`：HTTP 代理监听地址。
- `proxy.socks_addr`：SOCKS5 代理监听地址。
- `database.path`：SQLite 数据库文件位置。
- `runtime.max_attempts_per_request`：单次代理请求最多尝试几个节点。
- `runtime.failure_threshold`：节点连续失败几次后进入熔断。
- `runtime.circuit_break_seconds`：熔断持续时间。
- `admin.token`：管理后台 API Token。为空表示不开启后台鉴权，生产环境建议设置长随机字符串。

如果你把服务暴露到公网，请务必设置 `admin.token`，并优先使用防火墙、反向代理或内网访问控制保护后台。

## 页面说明

- 仪表盘：查看连接数、成功失败次数、流量、节点概况和 sing-box 状态。
- 节点：搜索、筛选、启用禁用、健康检查、查看节点详情。
- 订阅：添加订阅、刷新订阅、查看流量用量和到期时间。
- 分组：创建分组、管理分组下的节点。
- 关键词分组：按关键词批量自动分组。
- 凭证：创建代理账号，设置使用全部节点、分组或固定节点。
- 请求日志：查看代理请求失败原因和尝试过的节点。
- 操作日志：查看订阅刷新、凭证修改、批量操作等管理动作。
- 设置：查看当前服务监听地址和保存后台 Token。

## 常用命令

### 构建当前系统二进制

```bash
./scripts/build-release.sh
```

### 构建所有主流平台压缩包

```bash
./scripts/build-all-platforms.sh
```

默认输出到：

```text
release/packages
```

默认平台：

- Linux amd64 / arm64
- macOS amd64 / arm64
- Windows amd64 / arm64

### 发布 GitHub Release

如果你维护自己的 fork，可以使用：

```bash
./scripts/publish-release.sh v0.1.0
```

这个脚本会推送当前分支、创建版本标签、推送标签，并触发 GitHub Actions 自动打包发行版。

## API 简单示例

新增订阅：

```bash
curl -X POST http://127.0.0.1:8080/api/v1/subscriptions \
  -H 'Content-Type: application/json' \
  -d '{"name":"示例订阅","url":"https://example.com/sub","refresh_interval_seconds":3600}'
```

刷新订阅：

```bash
curl -X POST http://127.0.0.1:8080/api/v1/subscriptions/1/refresh
```

创建凭证：

```bash
curl -X POST http://127.0.0.1:8080/api/v1/credentials \
  -H 'Content-Type: application/json' \
  -d '{"username":"demo","password":"demo-pass","bind_mode":"all","selection_policy":"random"}'
```

查看系统健康：

```bash
curl http://127.0.0.1:8080/api/v1/system/health
```

如果设置了 `admin.token`，API 请求需要携带：

```bash
curl -H "Authorization: Bearer 你的token" http://127.0.0.1:8080/api/v1/system/health
```

## 故障排查

### 代理提示认证失败

请检查代理命令里的用户名和密码是否与“凭证”页面一致。

### SOCKS5 请求失败

请确认你连接的是 SOCKS5 端口，默认是 `127.0.0.1:1080`。

如果误把 SOCKS5 客户端连到 HTTP 端口 `1081`，可能会出现类似“invalid SOCKS5 response”的错误。

### 订阅刷新后没有节点

可以检查：

- 订阅链接是否能正常访问。
- User-Agent 是否需要改成 `clash/1.18.0`。
- 订阅返回的是否是 Clash YAML、Base64 或 URI 列表。

### 节点显示 sing-box 错误

打开节点详情查看 `sing_box_error`。

常见原因：

- 节点字段缺失。
- 当前构建没有带 `with_quic`，所以 Hysteria2 / TUIC 不可用。
- 当前构建没有带 `with_utls`，所以 REALITY 或特定 TLS fingerprint 不可用。

### 请求过程中遇到坏节点

系统会在同一次请求里自动换其他候选节点，默认最多尝试 `3` 个节点。

如果某个节点连续失败，会短暂进入熔断，默认 `60` 秒后恢复候选资格。

## 重要说明

- 当前主要面向本地或内网部署。
- 代理入口必须使用凭证认证。
- 后台鉴权默认关闭，生产环境请设置 `admin.token`。
- SQLite 适合个人和轻量团队使用，不建议把它当高并发中心数据库。
- 当前以 TCP 代理为主，不做 TUN、透明代理、系统路由接管。
- SOCKS5 UDP ASSOCIATE 当前不是主要目标。
- 引入 `github.com/sagernet/sing-box` 后，分发二进制时请自行确认并遵守相关开源许可证要求。

## 开发者说明

前端位于：

```text
web/
```

后端入口位于：

```text
cmd/jnmproxy
```

本地前端开发：

```bash
cd web
npm install
npm run dev
```

前端开发服务默认监听 `127.0.0.1:5173`，并把 `/api` 代理到后端 `127.0.0.1:8080`。

运行测试：

```bash
go test ./...
```

带 QUIC / uTLS 标签测试：

```bash
go test -tags "with_quic with_utls" ./...
```
