# Management

Management 是管理控制平面，负责管理控制台、用户与角色、网络与资源、策略配置、设备管理和审计。它维护期望状态，不承载普通用户业务流量。

## JRPC 接口

Management 使用 `pkg/jrpc` 提供的 HTTP JRPC 协议；HUB 的 Agent 长连接仍使用 gRPC，二者职责分开。

- 地址：`POST /jrpc`
- Content-Type：`application/json`
- 方法命名：`Service_Method`，例如 `System_Version`
- 参数放在 `params` 对象中；`id` 原样返回。
- 默认需要身份认证；仅显式标记为 public 的方法（当前 `System_Version`）允许匿名调用。
- 请求体限制默认 1 MiB。

```json
{"jsonrpc":"2.0","id":"req-001","method":"System_Version","params":{}}
```

当前模板 JRPC 实现的实际成功响应格式为：

```json
{"result":{"service":"management","version":"dev"},"error":null,"code":200,"id":"req-001"}
```

模板的前端文档示例声明了 `jsonrpc` 响应字段，但模板服务端当前未输出该字段，并额外输出 `code`。此处先保持与模板服务端代码兼容；不要将它描述为严格的 JSON-RPC 2.0 响应。后续如需改变线协议，应单独做兼容性版本设计。

## 目录职责

- `runtime/`：独立服务生命周期；未来可由组合启动入口与 HUB 一起运行。
- `internal/transport/jrpc/`：HTTP 监听、健康端点和 JRPC 服务注册。
- `internal/{identity,device,policy,network,...}/`：Management 业务模块，各模块通过 Repository 访问持久化数据。
- `configs/`：非敏感配置模板。
- `../pkg/jrpc/`：参考 Go 模板中的 `pkg/jrpc` 并适配到 monorepo 的协议运行时。

本阶段包含 JRPC 服务框架及公开版本查询方法；Management 身份认证服务、业务 Repository 和控制台方法尚未实现。没有配置 `IdentityResolver` 时，新增业务 RPC 默认拒绝匿名访问。

构建并启动独立 Management 服务：

```sh
make build-management
./bin/management --listen 127.0.0.1:8080
```

当前 HTTP 监听默认绑定回环地址。生产环境如需远程访问，应在受控网络中部署，并由反向代理提供 TLS；不要通过明文 HTTP 传输认证令牌。`cmd/server` 后续会复用 `management/runtime` 与 `hub/runtime`，合并部署时共享基础设施，但继续使用各自的监听端口和服务生命周期。
