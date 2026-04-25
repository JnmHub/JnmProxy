# JnmProxy 使用 sing-box 实现协议计划书

## 1. 计划目标

在不要求用户系统安装外部内核程序的前提下，引入 `github.com/sagernet/sing-box` 作为 Go 依赖，增强 JnmProxy 的上游节点协议支持能力。

本计划的核心目标：

- 保留 JnmProxy 现有能力：订阅、节点、分组、凭证、缓存、调度、统计、API。
- 使用 sing-box 负责复杂上游协议的连接能力。
- JnmProxy 继续作为统一入口，对外提供 HTTP 和 SOCKS5/SOCKS5H 代理入口。
- 客户端仍然必须使用 JnmProxy 凭证认证。
- 节点仍然由 JnmProxy 选择，sing-box 只负责连接被选中的上游节点。
- 不要求用户额外安装 `sing-box` 命令行程序。
- 不启动外部系统服务。
- 不把订阅规则、分流规则、策略组规则纳入 MVP。

## 2. 关键结论

### 2.1 `github.com/sagernet/sing-box` 的定位

`github.com/sagernet/sing-box` 本质上是 sing-box 的 Go 代理核心代码库。

如果我们在 Go 项目中直接依赖并编译它：

```text
JnmProxy 单进程
  ├── JnmProxy 管理层
  └── 内嵌 sing-box 协议能力
```

这不需要用户系统安装 sing-box，但从技术性质上看，属于“内嵌代理核心库”。

### 2.2 当前推荐落地方式

推荐采用：

```text
JnmProxy 自有入站代理
  -> 凭证认证
  -> 节点调度
  -> sing-box 出站适配器
  -> 目标站点
```

也就是：

- JnmProxy 仍然监听 `127.0.0.1:1081` HTTP 代理入口。
- JnmProxy 仍然监听 `127.0.0.1:1080` SOCKS5 代理入口。
- JnmProxy 自己完成认证、分组、随机/固定节点选择、流量统计。
- 选中节点后，交给 sing-box 出站适配层连接远端节点。

### 2.3 协议许可证风险

sing-box 使用 GPL 系列许可证。

如果 JnmProxy 直接链接 sing-box 并分发二进制，需要认真评估 GPL 对项目开源和分发方式的影响。

本计划必须在执行前确认：

- JnmProxy 是否接受 GPL 许可证约束。
- 是否允许后续分发的 JnmProxy 二进制遵循 GPL 要求。
- 是否需要在 README 中明确声明 sing-box 依赖及许可证。

如果不能接受 GPL 约束，本计划不能直接执行，必须改用其他许可证更友好的协议库方案。

## 3. 技术边界

### 3.1 保留 JnmProxy 自研的部分

以下能力继续由 JnmProxy 负责：

- 订阅链接管理。
- 订阅刷新定时任务。
- 节点解析与入库。
- 节点分组。
- 关键词自动分组。
- 凭证管理。
- 凭证绑定全部节点、分组或单节点。
- 代理入口认证。
- 节点调度。
- 连接级流量统计。
- SQLite 存储。
- REST API。
- 管理端后续前端接口。

### 3.2 交给 sing-box 的部分

以下能力交给 sing-box：

- Shadowsocks 出站。
- VMess 出站。
- VLESS 出站。
- Trojan 出站。
- Hysteria/Hysteria2 出站。
- TUIC 出站。
- WireGuard 出站。
- ShadowTLS 出站。
- AnyTLS 出站。
- Naive 出站。
- SSH 出站。
- 复杂传输层支持，如 WebSocket、gRPC、HTTPUpgrade、TLS、uTLS、REALITY 等。

### 3.3 不纳入本计划的部分

当前计划不做：

- 不使用 sing-box 的路由规则替代 JnmProxy 调度。
- 不把机场订阅规则导入 sing-box。
- 不开放 sing-box 原生管理接口。
- 不使用 sing-box 作为 JnmProxy 对外统一入口。
- 不做 TUN/透明代理。
- 不做系统级路由表修改。
- 不做 Android/iOS 移动端适配。
- 不默认支持 UDP 入站代理。

## 4. 当前系统现状

当前 JnmProxy 已实现：

- SQLite 自动迁移。
- 订阅 CRUD 和刷新。
- Clash YAML、Base64 URI、逐行 URI 解析。
- 节点统一入库。
- 分组与关键词分组。
- 凭证绑定与调度缓存。
- HTTP 和 SOCKS5 入站代理。
- 当前出站只真正支持：
  - `http`
  - `https`
  - `socks5`
- 流量统计内存聚合与延迟写库。
- 定时刷新和健康检查。
- REST API 和文档。

当前不足：

- `ss`、`vmess`、`vless`、`trojan` 等虽然能解析入库，但不能转发。
- `hysteria2`、`tuic`、`wireguard`、`anytls` 等尚未解析完善。
- 当前代理转发模型以 TCP 为主，UDP 能力未设计完成。

## 5. sing-box 支持范围目标

### 5.1 MVP 第一批支持

优先支持机场订阅最常见协议：

- `shadowsocks`
- `ss`
- `vmess`
- `vless`
- `trojan`
- `hysteria2`
- `hy2`
- `tuic`
- `http`
- `https`
- `socks`
- `socks5`
- `socks5h`

MVP 第一批必须做到：

- 可解析。
- 可转换为 sing-box outbound 配置。
- 可健康检查。
- 可通过 JnmProxy HTTP/SOCKS5 入站转发 TCP 流量。
- 可参与凭证调度。
- 可记录流量统计。

### 5.2 第二批支持

第二批支持复杂或较少见协议：

- `hysteria`
- `wireguard`
- `shadowtls`
- `anytls`
- `naive`
- `ssh`

第二批目标：

- 可入库。
- 可转换配置。
- TCP 能力优先。
- UDP 能力单独设计。

### 5.3 暂不承诺支持

以下协议暂不承诺完整支持：

- `ssr`
- 私有魔改协议。
- 非标准 URI 格式。
- 机场自定义字段。
- 需要系统 TUN 或路由权限才能正常工作的场景。

说明：

- sing-box 官方 outbound 类型中没有把 ShadowsocksR 作为主流出站目标。
- 如果后续必须支持 SSR，需要单独找库或继续标记为 `unsupported`。

## 6. 平台兼容计划

### 6.1 第一目标平台

优先支持：

- Linux `amd64`
- Linux `arm64`

原因：

- 最符合代理池后端部署场景。
- CI、构建、部署最稳定。
- 服务端运行成本最低。

### 6.2 第二目标平台

后续支持：

- Windows `amd64`
- Windows `arm64`
- macOS `amd64`
- macOS `arm64`

### 6.3 暂不作为后端目标平台

暂不处理：

- Android。
- iOS。
- OpenWrt 特殊架构。
- FreeBSD/OpenBSD。

这些平台可以作为后续独立适配阶段。

## 7. Go 版本与依赖计划

### 7.1 Go 版本

当前项目 `go.mod` 是 Go 1.22。

引入 `sing-box v1.13.x` 后，大概率需要升级 Go 版本。

计划要求：

- 将项目 Go 版本升级到 sing-box 当前版本要求的 Go 版本。
- 本地开发工具链同步升级。
- README 更新 Go 版本要求。
- CI 或测试命令统一使用新版本 Go。

### 7.2 依赖引入策略

优先方式：

```bash
go get github.com/sagernet/sing-box@v1.13.8
go mod tidy
go test ./...
```

原则：

- 不手动强行指定 `github.com/sagernet/sing` 版本，除非代码直接 import。
- 优先跟随 `sing-box` 官方 `go.mod` 的依赖组合。
- 如果确实需要直接使用 `github.com/sagernet/sing` 的底层接口，再显式依赖。

### 7.3 构建标签

根据 sing-box 实际功能需要，可能需要启用构建标签。

候选：

- QUIC/Hysteria/TUIC 相关构建标签。
- gRPC 相关构建标签。
- uTLS 相关构建标签。
- WireGuard 相关构建标签。

执行阶段必须通过实际编译确认，不提前假设。

## 8. 架构方案

### 8.1 总体架构

```text
客户端
  -> JnmProxy HTTP/SOCKS5 入站
  -> JnmProxy 凭证认证
  -> JnmProxy 调度缓存选择节点
  -> JnmProxy sing-box OutboundAdapter
  -> sing-box 协议出站
  -> 远端代理节点
  -> 目标网站
```

### 8.2 新增模块

```text
internal/singbox
  ├── config.go          # JnmProxy 节点转换为 sing-box outbound 配置
  ├── adapter.go         # JnmProxy OutboundAdapter 实现
  ├── engine.go          # sing-box 引擎生命周期管理
  ├── registry.go        # node_id -> adapter/engine 缓存
  ├── health.go          # 基于 sing-box 出站的健康检查
  ├── redactor.go        # sing-box 配置脱敏
  └── fixtures_test.go   # 协议配置测试夹具
```

### 8.3 现有模块改造

需要改造：

```text
internal/outbound
  └── 增加 sing-box 出站适配器，不删除现有 http/socks5 原生适配

internal/subscription
  └── 增强节点解析，生成 sing-box outbound JSON

internal/cache
  └── 缓存 sing-box 支持状态和 outbound 配置摘要

internal/scheduler
  └── 健康检查优先走 sing-box 适配器

internal/api
  └── 节点详情展示 sing-box 适配状态和错误
```

## 9. 两种落地模式

### 9.1 模式 A：直接 Dialer 适配，优先尝试

目标：

```text
JnmProxy OutboundAdapter
  -> sing-box outbound instance
  -> DialContext(network, destination)
```

优点：

- 架构最干净。
- 不需要本地临时端口。
- 和当前 `internal/outbound.Dialer` 最匹配。
- 流量统计仍由 JnmProxy 连接包装完成。

风险：

- sing-box 的内部 API 可能不稳定。
- 需要研究 outbound adapter 初始化方式。
- 可能涉及 sing-box option、registry、context、logger 初始化。
- 版本升级时维护成本较高。

验证标准：

- 能用 sing-box outbound 对一个 `ss` 节点完成 TCP Dial。
- 能用 sing-box outbound 对一个 `trojan` 或 `vmess` 节点完成 TCP Dial。
- 不需要启动完整 Box 路由系统。

### 9.2 模式 B：内嵌 Box + 本地回环入站，兜底方案

目标：

```text
JnmProxy
  -> 为节点生成 sing-box Box 配置
  -> Box 在本进程监听 127.0.0.1 随机端口
  -> JnmProxy 把被选节点当成本地 socks/http 上游
  -> sing-box 完成真实协议转发
```

优点：

- 更接近 sing-box 官方使用方式。
- 协议兼容性更高。
- 配置模型更稳定。
- 不依赖太深的内部 Dialer API。

缺点：

- 需要管理本地端口。
- 需要管理多个 Box 生命周期。
- 节点多时资源占用更大。
- 启动/销毁成本较高。

优化策略：

- 懒加载：节点首次被选中时启动对应 Box。
- LRU 缓存：限制同时活跃 Box 数量。
- 空闲回收：超过空闲时间自动关闭。
- 预热热门节点：分组或凭证常用节点可提前启动。
- 异常熔断：Box 启动失败时节点临时熔断。

### 9.3 模式选择规则

执行时必须按顺序：

1. 先技术验证模式 A。
2. 如果模式 A 可稳定编译和测试，则采用模式 A。
3. 如果模式 A API 不稳定或接入成本过高，则采用模式 B。
4. 不允许一开始同时实现两套完整方案。
5. 但代码接口必须为两种模式预留抽象。

## 10. 数据库改造计划

### 10.1 `proxy_nodes` 增加字段

新增字段：

- `sing_box_outbound_json`：转换后的 sing-box outbound 配置 JSON。
- `sing_box_status`：`supported`、`unsupported`、`error`。
- `sing_box_error`：最近一次配置转换或初始化错误。
- `sing_box_version`：生成配置时对应的 sing-box 版本。
- `udp_supported`：是否支持 UDP。
- `transport_type`：传输类型，如 `tcp`、`ws`、`grpc`、`httpupgrade`、`quic`。

保留字段：

- `adapter_status`：继续作为当前调度层是否可用的通用状态。
- `raw_config_json`：保留原始节点配置。
- `raw_uri`：保留原始 URI。

### 10.2 新增 `sing_box_engine_states`

用于模式 B 或未来调试。

字段：

- `id`
- `node_id`
- `engine_mode`：`dialer` 或 `box`
- `local_addr`
- `status`：`stopped`、`starting`、`running`、`failed`
- `last_error`
- `started_at`
- `last_used_at`
- `created_at`
- `updated_at`

### 10.3 迁移原则

- 必须写 SQLite migration。
- 老数据不丢失。
- 旧节点默认 `sing_box_status='unsupported'`，等待下次刷新或手动重建适配配置。
- 迁移后 `go test ./...` 必须通过。

## 11. 节点解析与转换计划

### 11.1 解析源

必须支持：

- Clash YAML `proxies`。
- Base64 URI 订阅。
- 逐行 URI 订阅。

暂不保存：

- Clash rules。
- proxy-groups。
- dns。
- rule-providers。

### 11.2 Clash YAML 字段映射

#### Shadowsocks

输入类型：

- `ss`
- `shadowsocks`

关键字段：

- `name`
- `server`
- `port`
- `cipher`
- `password`
- `plugin`
- `plugin-opts`
- `udp`

转换目标：

```json
{
  "type": "shadowsocks",
  "tag": "node-{id}",
  "server": "...",
  "server_port": 8388,
  "method": "...",
  "password": "..."
}
```

#### VMess

输入类型：

- `vmess`

关键字段：

- `uuid`
- `alterId`
- `cipher`
- `network`
- `tls`
- `servername`
- `ws-opts`
- `grpc-opts`
- `http-opts`

必须处理：

- TCP。
- WebSocket。
- gRPC。
- TLS。
- uTLS。

#### VLESS

输入类型：

- `vless`

关键字段：

- `uuid`
- `flow`
- `network`
- `tls`
- `servername`
- `reality-opts`
- `client-fingerprint`
- `ws-opts`
- `grpc-opts`

必须处理：

- TCP。
- WebSocket。
- gRPC。
- REALITY。
- Vision/XTLS 相关字段按 sing-box 支持程度映射。

#### Trojan

输入类型：

- `trojan`

关键字段：

- `password`
- `sni`
- `skip-cert-verify`
- `network`
- `ws-opts`
- `grpc-opts`

#### Hysteria2

输入类型：

- `hysteria2`
- `hy2`

关键字段：

- `password`
- `auth`
- `sni`
- `skip-cert-verify`
- `obfs`
- `obfs-password`
- `up`
- `down`

#### TUIC

输入类型：

- `tuic`

关键字段：

- `uuid`
- `password`
- `congestion-controller`
- `udp-relay-mode`
- `sni`
- `alpn`

#### HTTP/HTTPS

输入类型：

- `http`
- `https`

继续支持原生适配器，也可以统一转换为 sing-box outbound。

#### SOCKS5

输入类型：

- `socks`
- `socks5`
- `socks5h`

继续支持原生适配器，也可以统一转换为 sing-box outbound。

### 11.3 URI 格式解析

必须增强：

- `ss://`
- `vmess://`
- `vless://`
- `trojan://`
- `hysteria2://`
- `hy2://`
- `tuic://`
- `http://`
- `https://`
- `socks5://`
- `socks5h://`

URI 参数必须保留到 `raw_config_json`。

解析失败策略：

- 不让整个订阅刷新失败。
- 单个节点失败写入 refresh log 摘要。
- 失败节点不入库或以 `unsupported` 状态入库，具体在执行阶段确定。

### 11.4 敏感字段脱敏

日志和 API 默认必须脱敏：

- `password`
- `uuid`
- `id`
- `token`
- `private_key`
- `server_key`
- `short_id`
- `pbk`
- `auth`

## 12. OutboundAdapter 接口设计

### 12.1 新接口

```go
type Adapter interface {
    DialContext(ctx context.Context, node cache.NodeSnapshot, target string) (net.Conn, error)
    Check(ctx context.Context, node model.ProxyNode, target string) HealthResult
    Supports(protocol string) bool
    CloseNode(nodeID int64) error
}
```

### 12.2 适配器优先级

调度时按顺序：

1. 原生 HTTP/HTTPS/SOCKS5 适配器。
2. sing-box 适配器。
3. 不支持则返回错误并记录节点失败。

说明：

- 现有 HTTP/SOCKS5 原生出站代码保留，作为简单协议的轻量路径。
- sing-box 主要处理复杂协议。

### 12.3 错误处理

错误分类：

- 配置转换错误。
- sing-box 初始化错误。
- 上游握手失败。
- 目标连接失败。
- 超时。
- 协议不支持。

每种错误都要：

- 写入节点失败计数。
- 必要时触发临时熔断。
- 写入健康检查日志。
- 不泄露敏感参数。

## 13. 运行时生命周期设计

### 13.1 模式 A 生命周期

如果采用直接 Dialer：

- 启动时不预初始化所有节点。
- 节点第一次被选中时创建 outbound adapter。
- adapter 按节点 ID 缓存。
- 节点更新或订阅刷新后，关闭旧 adapter。
- adapter 长时间未使用则回收。

### 13.2 模式 B 生命周期

如果采用内嵌 Box：

- 每个活跃节点对应一个本地 Box 实例或共享 Box 实例。
- 每个 Box 使用随机本地端口。
- 本地端口只监听 `127.0.0.1`。
- Box 空闲超时后关闭。
- 节点配置变化后重建 Box。
- 程序退出时关闭所有 Box。

### 13.3 资源限制

必须支持配置：

- `max_active_engines`
- `engine_idle_timeout_seconds`
- `engine_start_timeout_seconds`
- `engine_dial_timeout_seconds`

默认建议：

```yaml
sing_box:
  enabled: true
  mode: "auto"
  max_active_engines: 64
  engine_idle_timeout_seconds: 600
  engine_start_timeout_seconds: 10
  engine_dial_timeout_seconds: 30
```

## 14. 配置文件改造

新增配置：

```yaml
sing_box:
  enabled: true
  version: "v1.13.8"
  mode: "auto" # auto | dialer | box
  prefer_native_http_socks: true
  max_active_engines: 64
  engine_idle_timeout_seconds: 600
  engine_start_timeout_seconds: 10
  engine_dial_timeout_seconds: 30
  log_level: "warn"
  health_check_target: "www.gstatic.com:443"
  enable_udp: false
```

说明：

- `mode=auto`：优先直接 Dialer，失败则使用 Box 兜底。
- `prefer_native_http_socks=true`：HTTP/SOCKS5 继续走 JnmProxy 原生实现。
- `enable_udp=false`：MVP 不开启 UDP。

## 15. 健康检查设计

### 15.1 TCP 健康检查

MVP 健康检查方式：

```text
通过被检查节点连接 health_check_target
```

默认目标：

```text
www.gstatic.com:443
```

成功标准：

- 能完成出站连接。
- 连接耗时记录为 `latency_ms`。
- 失败记录错误摘要。

### 15.2 HTTP 健康检查

后续增强：

- 通过节点请求 `https://www.gstatic.com/generate_204`。
- 验证 HTTP 状态码。
- 更准确判断代理是否可用。

### 15.3 UDP 健康检查

暂不作为 MVP。

后续如实现 UDP：

- DNS over UDP 检测。
- QUIC 连接检测。
- Hysteria2/TUIC 特定探测。

## 16. 流量统计兼容

当前 JnmProxy 统计方式继续保留：

- 入站连接包装。
- 连接开始记录连接数。
- 转发时统计上下行字节。
- 连接结束记录成功/失败。
- 内存聚合。
- 延迟写入 SQLite。

使用 sing-box 后仍然由 JnmProxy 统计：

```text
客户端 <-> JnmProxy <-> sing-box outbound <-> 目标
```

因为流量仍经过 JnmProxy 的连接管道，所以统计逻辑不需要整体推翻。

## 17. UDP 支持计划

### 17.1 当前状态

当前 JnmProxy 主要支持 TCP：

- HTTP 代理。
- HTTPS CONNECT。
- SOCKS5 CONNECT。

### 17.2 UDP 后续目标

如果需要完整代理能力，后续增加：

- SOCKS5 UDP ASSOCIATE。
- UDP 流量统计。
- UDP 节点健康检查。
- sing-box UDP 出站调用。

### 17.3 UDP 不阻塞 MVP

MVP 先完成 TCP 出站，UDP 后续单独阶段。

原因：

- 大多数 HTTP/HTTPS 代理使用 TCP。
- 机场节点先做到 TCP 可用即可满足主要代理池需求。
- UDP 涉及 NAT、会话、超时、统计维度，应该单独设计。

## 18. API 改造

### 18.1 节点详情新增字段

节点 API 新增：

- `sing_box_status`
- `sing_box_error`
- `sing_box_version`
- `transport_type`
- `udp_supported`

### 18.2 节点操作新增接口

新增：

- `POST /api/v1/nodes/{id}/rebuild-adapter`
- `POST /api/v1/nodes/rebuild-adapters`
- `GET /api/v1/sing-box/status`
- `POST /api/v1/sing-box/reload`

### 18.3 系统状态新增

`GET /api/v1/system/health` 增加：

- sing-box 是否启用。
- sing-box 模式。
- 活跃 engine 数量。
- sing-box 版本。

## 19. 开发阶段计划

### 阶段 1：许可证与技术可行性确认

任务：

- 确认 GPL 许可证是否可接受。
- 升级 Go 工具链测试。
- 引入 `github.com/sagernet/sing-box` 依赖。
- 验证项目能编译。
- 写一个最小 sing-box outbound 连接实验。
- 决定采用模式 A 或模式 B。

验收：

- 明确许可证决策。
- `go test ./...` 可运行。
- 至少一个 sing-box 出站实验成功。
- 计划中模式选择结论落到文档。

### 阶段 2：数据库与配置迁移

任务：

- 增加 `sing_box` 配置。
- 增加数据库迁移。
- 增加节点 sing-box 字段。
- 增加 engine 状态表。
- 更新 config example。
- 更新 README Go 版本要求。

验收：

- 老数据库可迁移。
- 新数据库可初始化。
- 配置默认值合理。
- 测试通过。

### 阶段 3：节点转换器

任务：

- 实现 Clash YAML 到 sing-box outbound JSON 转换。
- 实现 URI 到 sing-box outbound JSON 转换。
- 实现敏感字段脱敏。
- 增加协议 fixture。
- 增加转换单元测试。

验收：

- `ss` 转换成功。
- `vmess` 转换成功。
- `vless` 转换成功。
- `trojan` 转换成功。
- `hysteria2` 转换成功。
- `tuic` 转换成功。
- 失败节点不会影响整个订阅刷新。

### 阶段 4：sing-box OutboundAdapter

任务：

- 定义统一 Adapter 接口。
- 接入 sing-box adapter。
- 保留原生 HTTP/SOCKS5 adapter。
- 实现 adapter registry。
- 实现节点配置变化后的 adapter 失效。
- 实现 adapter 空闲回收。

验收：

- 当前 HTTP/SOCKS5 功能不退化。
- sing-box 支持节点能被调度。
- 不支持节点不进入调度。
- 节点配置更新后 adapter 会重建。

### 阶段 5：核心协议转发

任务：

- 打通 `ss` TCP 转发。
- 打通 `trojan` TCP 转发。
- 打通 `vmess` TCP/WS/TLS 转发。
- 打通 `vless` TCP/WS/TLS/REALITY 转发。
- 打通 `hysteria2` TCP 代理能力。
- 打通 `tuic` TCP 代理能力。

验收：

- 每种协议至少有一个可运行集成测试或手动验证记录。
- HTTP 代理入口可使用这些节点。
- SOCKS5 代理入口可使用这些节点。
- 流量统计正常。
- 失败节点会触发失败计数和熔断。

### 阶段 6：健康检查与调度融合

任务：

- 健康检查使用 sing-box adapter。
- 检查结果写入 `node_health_checks`。
- 不可用节点不参与调度。
- 熔断后可重新检查恢复。
- API 展示 sing-box 错误摘要。

验收：

- 健康检查能覆盖 sing-box 节点。
- 死节点不会被选中。
- 恢复后节点可重新参与调度。

### 阶段 7：订阅刷新联动

任务：

- 订阅刷新时生成 sing-box outbound JSON。
- 节点新增时建立适配状态。
- 节点更新时标记 adapter 重建。
- 节点消失时关闭相关 adapter/engine。
- 刷新日志记录协议转换统计。

验收：

- 一个订阅里不同协议节点能混合入库。
- 支持节点进入调度。
- 不支持节点保留但不调度。
- 刷新日志能看出成功、失败和 unsupported 数量。

### 阶段 8：API 与文档完善

任务：

- 更新 API 文档。
- 更新 README。
- 增加 sing-box 状态 API。
- 增加节点重建 adapter API。
- 增加故障排查说明。
- 增加许可证说明。

验收：

- 前端可展示 sing-box 状态。
- 用户可通过 API 重建节点适配器。
- 文档说明哪些协议支持、哪些暂不支持。

### 阶段 9：稳定性测试

任务：

- 并发代理请求测试。
- 多协议混合订阅测试。
- 长连接测试。
- 节点失败熔断测试。
- 订阅刷新期间代理不中断测试。
- adapter/engine 空闲回收测试。
- 内存泄漏初步观察。

验收：

- `go test ./...` 通过。
- 长连接不中断或失败可控。
- 订阅刷新不会导致全部 adapter 崩溃。
- engine 回收后可重新启动。

## 20. 验收标准

本计划完成时必须满足：

- JnmProxy 不要求系统安装 sing-box 命令。
- sing-box 作为 Go 依赖被编译进程序或同进程使用。
- HTTP/SOCKS5 入站认证逻辑仍由 JnmProxy 控制。
- 节点调度仍由 JnmProxy 控制。
- 流量统计仍由 JnmProxy 控制。
- 至少支持 `ss`、`vmess`、`vless`、`trojan`、`hysteria2`、`tuic` 的 TCP 转发。
- `http`、`https`、`socks5` 现有支持不退化。
- 未支持协议不会进入调度。
- 节点健康检查能覆盖 sing-box 节点。
- API 能展示 sing-box 状态和节点适配状态。
- README 明确 Go 版本、sing-box 依赖和许可证风险。
- 全量测试通过。

## 21. 风险与应对

### 21.1 许可证风险

风险：

- GPL 可能影响 JnmProxy 分发方式。

应对：

- 执行前确认是否接受 GPL。
- README 和文档明确依赖和许可证。
- 如果不接受，停止本计划，改用其他依赖方案。

### 21.2 API 不稳定风险

风险：

- sing-box 内部 Go API 不是专门为第三方稳定嵌入设计。

应对：

- 先做技术验证。
- 优先封装在 `internal/singbox`，避免污染业务层。
- 预留模式 B 兜底。
- 锁定 sing-box 版本。

### 21.3 构建复杂风险

风险：

- QUIC、uTLS、WireGuard 等能力可能依赖构建标签或 CGO。

应对：

- 第一目标平台只做 Linux amd64/arm64。
- 每个协议单独测试。
- README 明确构建要求。

### 21.4 资源占用风险

风险：

- 模式 B 多节点 Box 实例可能占用较多内存和端口。

应对：

- 限制最大活跃 engine。
- 空闲回收。
- LRU 淘汰。
- 节点失败快速熔断。

### 21.5 协议字段兼容风险

风险：

- 机场订阅字段不统一。

应对：

- 保留原始配置。
- 转换器容错。
- 不支持字段记录警告。
- fixture 持续补充。

## 22. 执行纪律

执行本计划时必须遵守：

- 不同时大改多个主线模块。
- 每完成一个小阶段并测试通过后提交 Git。
- 每个阶段都运行 `go test ./...`。
- 遇到 sing-box 接入阻塞时，先记录问题并回到本计划主线。
- 不借机重写前端。
- 不借机改动无关业务逻辑。
- 不在日志中输出节点密码、UUID、token、订阅 URL 敏感部分。

## 23. 官方参考

- sing-box Outbound 官方文档：https://sing-box.sagernet.org/configuration/outbound/
- sing-box 构建文档：https://sing-box.sagernet.org/installation/build-from-source/
- sing-box GitHub 仓库：https://github.com/SagerNet/sing-box
- sing-box License：https://github.com/SagerNet/sing-box/blob/main/LICENSE

