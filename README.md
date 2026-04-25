# JnmProxy

JnmProxy 是一个 Go 代理池系统，基于机场订阅链接维护代理节点，提供 HTTP/SOCKS5 入站代理、凭证认证、分组调度、订阅刷新、流量统计和单页管理后台。

## 功能状态

- SQLite 本地数据库，启动自动迁移。
- 多订阅管理，刷新订阅时默认使用 `User-Agent: clash.meta`，遇到不支持客户端提示节点时会自动重试默认 UA。
- 支持 Clash YAML、Base64 URI、逐行 URI 订阅解析。
- 所有节点统一入库，并保留订阅来源标识。
- 支持手动分组、批量分组、关键词自动分组。
- 支持多个代理访问凭证，凭证可绑定全部节点、分组或单个节点。
- HTTP 和 SOCKS5 入站代理强制用户名密码认证。
- 原生出站适配支持 HTTP、HTTPS、SOCKS5 上游代理。
- 已内嵌 `github.com/sagernet/sing-box v1.13.8` 作为 Go 依赖，不要求系统额外安装 sing-box 命令。
- sing-box 出站适配默认支持 `ss`、`shadowsocks`、`vmess`、`vless`、`trojan`、`http`、`https`、`socks`、`socks5`、`socks5h` 的 TCP 转发链路。
- `hysteria2`、`hy2`、`tuic` 依赖 QUIC，构建或测试时需要增加 `-tags with_quic`。
- 订阅刷新会为节点生成 `sing_box_outbound_json`、`sing_box_status`、`sing_box_error`、`sing_box_version`、`udp_supported`、`transport_type` 等适配字段。
- 流量统计先写内存，再定时批量写入 SQLite。
- 代理请求遇到坏节点时，会在同一次请求内自动换另一个节点重试，默认最多尝试 3 个不同节点。
- 节点连续失败会进入内存熔断，默认连续失败 3 次后临时避开 60 秒。
- 节点页可查看运行态候选池、连续失败、熔断恢复时间和最近失败原因。
- 新增代理请求失败日志，能看到入口协议、凭证、目标地址、尝试节点、失败原因和耗时。
- 顶部全局搜索支持节点、订阅、分组、凭证、操作日志和请求日志快速跳转。
- 定时刷新订阅和节点健康检查已接入，健康检查可覆盖 sing-box 节点。
- 提供 `/api/v1` 后端管理 API。
- Go 后端会托管内嵌的前端管理后台，访问 `http://127.0.0.1:8080/` 即可打开页面。
- 管理 API 支持可选 Bearer Token；`admin.token` 为空时保持本地开发友好模式。

## 本地启动

如果系统没有 `go` 命令，需要先安装 Go 1.24.7+。

```bash
go test ./...
go run ./cmd/jnmproxy -migrate-only
go run ./cmd/jnmproxy
```

如果需要 Hysteria2/TUIC、REALITY 或带 `client-fingerprint` 的 TLS 节点：

```bash
go test -tags "with_quic with_utls" ./internal/singbox -run 'TestBuildOutboundForCoreProtocols|TestQUICProtocolTCPTransfers' -count=1 -v
go run -tags "with_quic with_utls" ./cmd/jnmproxy
```

默认监听：

- 管理后台/API：`127.0.0.1:8080`
- HTTP 代理：`127.0.0.1:1081`
- SOCKS5 代理：`127.0.0.1:1080`
- SQLite：`./data/jnmproxy.db`

可复制 `config.example.yaml` 为 `config.yaml` 后调整配置。

## 运行态兜底配置

`runtime` 配置控制“坏节点自动换”和“内存熔断”：

```yaml
runtime:
  max_attempts_per_request: 3
  failure_threshold: 3
  circuit_break_seconds: 60
  record_failed_requests: true
```

- `max_attempts_per_request`：同一次 HTTP CONNECT / SOCKS5 请求最多尝试几个不同节点。
- `failure_threshold`：同一个节点连续失败多少次后进入短期熔断。
- `circuit_break_seconds`：熔断持续多久，时间到后会重新进入候选池。
- `record_failed_requests`：是否记录代理请求最终失败日志，默认只记失败请求，避免成功请求过多写库。

## 前端开发

前端是 `web/` 目录下的 React + TypeScript + Vite 单页管理后台。

开发启动：

```bash
cd web
npm install
npm run dev
```

默认前端开发服务监听 `127.0.0.1:5173`，并把 `/api` 代理到后端 `127.0.0.1:8080`。

生产构建：

```bash
cd web
npm run build
```

当前 Go 程序内嵌托管的静态文件位于 `internal/webui/dist`。更新前端生产包后，需要把 `web/dist` 同步到 `internal/webui/dist`，再重新构建 Go 程序。

一键打包前后端：

```bash
./scripts/build-release.sh
```

脚本会依次执行：

- 构建 `web/dist`。
- 同步到 `internal/webui/dist`。
- 执行 `go test -tags "with_quic with_utls" ./...`。
- 编译二进制到 `bin/jnmproxy`。

可用环境变量覆盖默认值：

```bash
GO_TAGS="" OUTPUT=./bin/jnmproxy ./scripts/build-release.sh
```

全平台打包：

```bash
./scripts/build-all-platforms.sh
```

默认输出到 `release/packages`，包含：

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`
- `windows/amd64`
- `windows/arm64`

全平台脚本默认使用：

```bash
GO_TAGS="sqlite_purego with_quic with_utls"
CGO_ENABLED=0
```

这样可以在 Linux 上直接交叉编译 macOS、Windows、Linux 包，不需要系统安装 Darwin/Windows 交叉 C 编译器。

如果要只构建不重新打前端、不跑测试：

```bash
RUN_WEB_BUILD=0 RUN_TESTS=0 ./scripts/build-all-platforms.sh
```

如果希望所有平台都打成 `.zip`：

```bash
PACKAGE_FORMAT=zip ./scripts/build-all-platforms.sh
```

## GitHub Actions 自动发行版

仓库已内置 `.github/workflows/release.yml`。

创建并推送版本标签后，会自动：

- 安装 Go 和 Node.js。
- 构建前端并同步到 Go 内嵌目录。
- 使用 `sqlite_purego with_quic with_utls` 标签执行全量测试。
- 交叉编译 Linux、macOS、Windows 的 amd64/arm64 包。
- 将所有平台包打成 `.zip`。
- 生成 `SHA256SUMS`。
- 创建 GitHub Release 并上传附件。

推荐发布命令：

```bash
git tag v0.1.0
git push origin v0.1.0
```

也可以在 GitHub 页面进入 `Actions -> Release -> Run workflow`，手动填写版本号创建发行版。

## SQLite 驱动说明

项目支持两种 SQLite 驱动：

- 默认构建：`github.com/mattn/go-sqlite3`，成熟稳定，性能通常更好，但依赖 CGO，本机需要 C 编译器，跨平台打包更麻烦。
- 全平台打包：`modernc.org/sqlite`，纯 Go 实现，通过 `sqlite_purego` 构建标签启用，不依赖 CGO，适合一次打 macOS/Windows/Linux 多平台包。

两者使用同一个 SQLite 数据库文件格式，表结构和迁移逻辑一致。正常功能没有区别；主要影响是“怎么编译”和极端高并发场景下的性能差异。本项目已限制 SQLite 单连接写入，管理后台和本地代理池场景优先选择可部署性更好的纯 Go 打包方式。

## API 示例

新增订阅：

```bash
curl -X POST http://127.0.0.1:8080/api/v1/subscriptions \
  -H 'Content-Type: application/json' \
  -d '{"name":"示例订阅","url":"https://example.com/sub","refresh_interval_seconds":3600}'
```

手动刷新订阅：

```bash
curl -X POST http://127.0.0.1:8080/api/v1/subscriptions/1/refresh
```

查看 sing-box 运行配置摘要：

```bash
curl http://127.0.0.1:8080/api/v1/system/sing-box
```

如果配置了 `admin.token`，管理 API 需要携带：

```bash
curl -H "Authorization: Bearer <你的token>" http://127.0.0.1:8080/api/v1/system/health
```

重建单个节点的 sing-box 适配器：

```bash
curl -X POST http://127.0.0.1:8080/api/v1/nodes/1/rebuild-adapter
```

创建凭证：

```bash
curl -X POST http://127.0.0.1:8080/api/v1/credentials \
  -H 'Content-Type: application/json' \
  -d '{"username":"15376259491","password":"00hhg5210","bind_mode":"all","selection_policy":"random"}'
```

使用 HTTP 代理：

```bash
curl -x http://15376259491:00hhg5210@127.0.0.1:1081 https://example.com
```

使用 SOCKS5H 代理：

```bash
curl --proxy socks5h://15376259491:00hhg5210@127.0.0.1:1080 https://example.com
```

## 重要说明

- 不引入 Clash 内核。
- 已引入 `github.com/sagernet/sing-box` Go 依赖作为内嵌协议能力；sing-box 使用 GPL 系列许可证，分发二进制前需要遵守对应许可证要求。
- 不保存订阅规则、分流规则和策略组规则。
- 未支持的节点协议会入库，但默认不参与代理调度。
- 当前 MVP 以 TCP 代理为主，SOCKS5 UDP ASSOCIATE、透明代理、TUN 和系统路由不在本阶段范围内。
- QUIC 协议族节点只有在 `with_quic` 构建标签启用时才进入 sing-box 支持状态，否则会记录 `sing_box_status=error` 并提示重新构建。
- REALITY 或带 `client-fingerprint` 的 TLS 节点需要 `with_utls`，否则会记录 `sing_box_status=error` 并提示重新构建。
- 管理 API 默认建议只监听本地地址；如需额外保护，请设置 `admin.token`，前端设置页会把 token 保存到当前浏览器的 `localStorage`。

## sing-box 故障排查

- `sing_box_status=error`：查看节点的 `sing_box_error`，通常是订阅字段缺失、协议字段不兼容或 sing-box 配置无法初始化。
- `alive_status=dead`：健康检查无法通过该节点连接目标，节点会暂时从运行时缓存候选中移除，后续健康检查恢复后会重新进入调度。
- 节点订阅更新后仍异常：调用 `POST /api/v1/nodes/{id}/rebuild-adapter` 关闭该节点旧适配器，下次请求会按最新配置重建。
- HTTP/HTTPS/SOCKS5 简单协议默认优先走 JnmProxy 原生出站；复杂协议走 sing-box 出站。
