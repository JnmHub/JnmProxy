# sing-box 阶段 1 决策记录

## 决策结论

本项目继续执行 `使用sing-box实现协议计划书.md`，采用 `github.com/sagernet/sing-box v1.13.8` 作为 Go 依赖引入。

阶段 1 结论：

- 许可证：接受引入 sing-box 带来的 GPL 许可证约束风险，后续 README 和发布说明必须明确声明。
- Go 版本：项目工具链升级到 Go `1.24.7`。
- sing-box 版本：锁定为 `github.com/sagernet/sing-box v1.13.8`。
- sing 版本：跟随 sing-box 官方依赖，目前为 `github.com/sagernet/sing v0.8.4`，不单独强行指定更高版本。
- 适配模式：优先采用计划书中的模式 A，即直接通过 sing-box `OutboundManager` 获取出站并调用 `DialContext`。
- 兜底模式：保留模式 B，即内嵌 Box + 本地回环入站，仅在复杂协议无法稳定用模式 A 接入时启用。

## 验证结果

已新增最小实验测试：

- 文件：`internal/singbox/experiment_test.go`
- 测试：`TestEmbeddedBoxDirectOutboundDial`
- 验证内容：
  - 使用 `include.Context` 注册 sing-box 入站/出站协议能力。
  - 使用 JSON 配置创建内嵌 `box.Box`。
  - 启动 sing-box 实例。
  - 通过 `instance.Outbound().Outbound("direct-out")` 获取出站。
  - 调用 sing-box 出站的 `DialContext` 连接本地 HTTP 测试服务。
  - 完成 HTTP 请求并验证响应。

通过命令：

```bash
go test ./internal/singbox -run TestEmbeddedBoxDirectOutboundDial -count=1 -v
```

## 后续执行要求

后续阶段必须继续遵守：

- 不让 sing-box 接管 JnmProxy 的客户端入站认证。
- 不让 sing-box 接管 JnmProxy 的节点调度。
- 不导入机场订阅规则、分流规则、策略组规则。
- 所有复杂协议能力都封装在 `internal/singbox` 或 `internal/outbound` 的适配层中。
- HTTP/HTTPS/SOCKS5 原生出站能力继续保留。
- 每完成小阶段且测试正常后提交 Git。

