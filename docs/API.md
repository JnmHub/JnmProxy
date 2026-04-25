# JnmProxy API 文档

API 前缀：`/api/v1`

当前 API 默认建议只监听 `127.0.0.1`。返回格式统一为 JSON，失败时返回：

```json
{"error":"错误说明"}
```

## 系统

### `GET /search?q=关键词`

全局搜索节点、订阅、分组、凭证、操作日志和请求日志。

响应：

```json
{
  "query": "香港",
  "items": [
    {
      "type": "node",
      "id": 1,
      "title": "香港 HK 01",
      "subtitle": "节点 / vmess / 1.2.3.4:443",
      "url": "/nodes?search=香港"
    }
  ]
}
```

### `GET /system/health`

返回系统健康状态。

响应：

```json
{"status":"ok","time":"2026-04-24T12:00:00Z"}
```

### `GET /system/sing-box`

返回 sing-box 内嵌适配状态和配置摘要。

响应：

```json
{
  "enabled": true,
  "version": "v1.13.8",
  "config_version": "v1.13.8",
  "mode": "auto",
  "prefer_native_http_socks": true,
  "adapter_configured": true,
  "max_active_engines": 64,
  "engine_idle_timeout_seconds": 600,
  "engine_dial_timeout_seconds": 30,
  "health_check_target": "www.gstatic.com:443",
  "enable_udp": false,
  "quic_enabled": true,
  "utls_enabled": true,
  "supported_protocols": ["ss", "vmess", "vless", "trojan", "hysteria2", "tuic"],
  "license": "GPL via github.com/sagernet/sing-box"
}
```

## 订阅

### `POST /subscriptions`

创建订阅。

请求：

```json
{
  "name": "示例订阅",
  "url": "https://example.com/sub",
  "user_agent": "clash.meta",
  "refresh_interval_seconds": 3600,
  "enabled": true
}
```

### `GET /subscriptions`

订阅列表。

### `GET /subscriptions/{id}`

订阅详情。

### `PUT /subscriptions/{id}`

更新订阅，字段均可选。

### `DELETE /subscriptions/{id}`

删除订阅，同时级联删除该订阅节点。

### `POST /subscriptions/{id}/refresh`

手动刷新订阅。

响应：

```json
{
  "subscription_id": 1,
  "node_count": 12,
  "http_status": 200,
  "sing_box_supported_count": 10,
  "sing_box_error_count": 1,
  "unsupported_count": 1
}
```

### `GET /subscriptions/{id}/refresh-logs`

订阅刷新日志，包含 sing-box 转换统计：

- `sing_box_supported_count`
- `sing_box_error_count`
- `unsupported_count`

### `GET /subscriptions/{id}/nodes`

查看某订阅下的节点。

## 节点

### `GET /runtime/nodes`

查看内存里的节点运行态。

响应字段：

- `node_id`：节点 ID。
- `failure_count`：当前内存连续失败次数。
- `circuit_open`：是否正在短期熔断。
- `circuit_until`：熔断恢复时间。
- `in_candidate_pool`：当前是否还在运行候选池。
- `last_failure`：最近一次代理失败原因。
- `last_failed_at`：最近一次代理失败时间。

### `GET /nodes`

节点列表，支持查询参数：

- `subscription_id`
- `group_id`
- `protocol`
- `alive_status`
- `enabled`

### `GET /nodes/{id}`

节点详情。

节点对象包含 sing-box 适配字段：

- `sing_box_outbound_json`
- `sing_box_status`：`supported`、`unsupported`、`error`
- `sing_box_error`
- `sing_box_version`
- `udp_supported`
- `transport_type`

### `PUT /nodes/{id}`

更新节点启用状态。

请求：

```json
{"enabled":false}
```

### `POST /nodes/{id}/check`

检查单个节点健康状态。

### `POST /nodes/{id}/rebuild-adapter`

关闭该节点已缓存的 sing-box 适配器，下次使用节点时按数据库最新配置重建。

响应：

```json
{"node_id":1,"status":"adapter_rebuild_scheduled"}
```

### `POST /nodes/check`

批量检查所有可检查节点。

### `POST /nodes/batch`

批量操作节点。

请求：

```json
{"action":"add_group","node_ids":[1,2],"group_id":3}
```

支持动作：

- `enable`
- `disable`
- `add_group`
- `remove_group`

## 代理请求日志

### `GET /proxy-request-logs`

查看代理请求最终失败日志，支持分页和搜索。

查询参数：

- `page`
- `page_size`
- `search`
- `status`
- `entry_protocol`

响应：

```json
{
  "items": [
    {
      "id": 1,
      "entry_protocol": "SOCKS5",
      "credential_id": 1,
      "username": "user",
      "target_address": "example.com:443",
      "status": "failed",
      "attempt_count": 2,
      "selected_node_id": 0,
      "selected_node_name": "",
      "error": "dial failed",
      "attempts_json": "[{\"node_id\":1,\"node_name\":\"香港 HK 01\",\"success\":false,\"error\":\"dial failed\"}]",
      "duration_ms": 120,
      "created_at": "2026-04-25T12:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 50
}
```

## 分组

### `POST /groups`

创建分组。

请求：

```json
{"name":"香港","description":"香港节点"}
```

### `GET /groups`

分组列表。

### `GET /groups/{id}`

分组详情。

### `PUT /groups/{id}`

更新分组。

### `DELETE /groups/{id}`

删除分组。

### `POST /groups/{id}/nodes`

批量添加节点到分组。

```json
{"node_ids":[1,2,3]}
```

### `DELETE /groups/{id}/nodes`

批量移出节点。

```json
{"node_ids":[1,2,3]}
```

## 关键词分组

### `POST /group-keywords`

创建关键词规则。

```json
{"name":"地区规则","keywords":"香港|HK|日本","case_sensitive":false,"enabled":true}
```

### `GET /group-keywords`

关键词规则列表。

### `PUT /group-keywords/{id}`

更新关键词规则。

### `DELETE /group-keywords/{id}`

删除关键词规则。

### `POST /group-keywords/apply`

执行关键词自动分组。

```json
{"all":true}
```

或：

```json
{"rule_ids":[1,2],"all":false}
```

## 凭证

### `POST /credentials`

创建代理访问凭证。

```json
{
  "username": "15376259491",
  "password": "00hhg5210",
  "enabled": true,
  "bind_mode": "group",
  "selection_policy": "random",
  "remark": "示例凭证",
  "bindings": [
    {"target_type":"group","target_id":1}
  ]
}
```

## sing-box 协议说明

当前后端不要求系统安装 sing-box 命令，协议能力来自 Go 依赖 `github.com/sagernet/sing-box v1.13.8`。

MVP 第一批 TCP 出站支持：

- `ss`
- `shadowsocks`
- `vmess`
- `vless`
- `trojan`
- `http`
- `https`
- `socks`
- `socks5`
- `socks5h`

启用 `-tags with_quic` 构建后额外支持：

- `hysteria2`
- `hy2`
- `tuic`

未启用 `with_quic` 时，Hysteria2/TUIC 节点会保留在数据库中，但转换结果为 `sing_box_status=error`，不会进入调度。

REALITY 或带 `client-fingerprint` 的 TLS 节点需要启用 `with_utls`，建议生产启动使用：

```bash
go run -tags "with_quic with_utls" ./cmd/jnmproxy
```

当前不开放 sing-box 原生管理接口，不导入机场规则、分流规则和策略组规则。HTTP/SOCKS5 入站认证、节点调度、健康检查和流量统计仍由 JnmProxy 控制。

## 故障排查

- `sing_box_status=error`：查看 `sing_box_error`，常见原因是订阅字段缺失、协议字段不兼容或配置无法初始化。
- `alive_status=dead`：健康检查失败，该节点不会进入运行时调度缓存；恢复后会重新参与调度。
- 节点更新后仍走旧配置：调用 `POST /nodes/{id}/rebuild-adapter`。
- 当前 UDP 入站代理、TUN、透明代理和系统路由不属于 MVP。

字段说明：

- `bind_mode`：`all`、`group`、`node`
- `selection_policy`：`random`、`fixed`
- `bindings[].target_type`：`group` 或 `node`

### `GET /credentials`

凭证列表，不返回密码哈希。

### `GET /credentials/{id}`

凭证详情，不返回密码哈希。

### `PUT /credentials/{id}`

更新凭证启用状态、绑定范围、选择策略和备注。

### `POST /credentials/{id}/reset-password`

重置密码。

```json
{"password":"new-password"}
```

### `DELETE /credentials/{id}`

删除凭证。

## 统计

### `GET /stats/overview`

总体流量统计。查询前会先 flush 当前内存统计到 SQLite。

响应：

```json
{
  "connections": 10,
  "success_connections": 9,
  "failed_connections": 1,
  "upload_bytes": 1234,
  "download_bytes": 5678
}
```
