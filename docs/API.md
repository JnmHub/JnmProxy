# JnmProxy API 文档

API 前缀：`/api/v1`

当前 API 默认建议只监听 `127.0.0.1`。返回格式统一为 JSON，失败时返回：

```json
{"error":"错误说明"}
```

## 系统

### `GET /system/health`

返回系统健康状态。

响应：

```json
{"status":"ok","time":"2026-04-24T12:00:00Z"}
```

## 订阅

### `POST /subscriptions`

创建订阅。

请求：

```json
{
  "name": "示例订阅",
  "url": "https://example.com/sub",
  "user_agent": "clash/1.18.0",
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
{"subscription_id":1,"node_count":12,"http_status":200}
```

### `GET /subscriptions/{id}/refresh-logs`

订阅刷新日志。

### `GET /subscriptions/{id}/nodes`

查看某订阅下的节点。

## 节点

### `GET /nodes`

节点列表，支持查询参数：

- `subscription_id`
- `group_id`
- `protocol`
- `alive_status`
- `enabled`

### `GET /nodes/{id}`

节点详情。

### `PUT /nodes/{id}`

更新节点启用状态。

请求：

```json
{"enabled":false}
```

### `POST /nodes/{id}/check`

检查单个节点健康状态。

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

