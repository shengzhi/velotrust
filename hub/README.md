# HUB

HUB 是运行控制平面，负责节点认证、注册凭证校验、配置与策略同步、会话管理、Peer 状态及运行状态上报。实际业务流量由 Gateway 和 Client Agent 直接通过隧道传输，不经过 HUB。

## 当前代码框架

- `config/`：HUB 监听地址、稳定实例标识和 TLS 配置。
- `runtime/`：服务生命周期和优雅关闭，可由独立入口或后续 `cmd/server` 组合启动。
- `transport/grpc/`：gRPC 服务端、TLS 与标准健康检查。
- `internal/auth/`：节点身份认证接口。
- `internal/session/`：节点会话和实例租约存储接口。
- `internal/configsync/`：节点期望配置读取接口及独立于 Management 数据实体的配置快照。
- `internal/report/`：节点运行状态上报接口。
- `shared/proto/v1/hub.proto`：跨进程控制通道契约；生成代码位于 `shared/gen/go/hub/v1/`。

目前只有 TLS gRPC 监听和健康检查可运行。节点注册、认证实现、双向控制流处理、数据库 Repository 和多实例会话协调尚未实现；控制流契约与业务接口先行定义，不代表这些业务已可用。

## 协议选择

HUB 南向控制通道采用 **gRPC over HTTP/2 + Protocol Buffers**。节点连接后使用双向流承载心跳、配置下发、应用回执和状态上报；注册、诊断等请求使用 unary RPC。TLS 是传输层最低要求，节点认证和凭证撤销由 HUB 认证层负责，不能把“连上 TLS”视为节点已授权。

Go 标准库 `net/rpc` 不作为候选：它是已冻结的通用 RPC 包，缺少本项目需要的双向流和跨语言协议生成能力。自定义 TCP 会要求项目自行维护消息 framing、版本兼容、重连、背压、流控、超时及认证集成；除非未来有明确的 HTTP/2 不可用环境或性能证据，否则不承担这套协议维护成本。

## 本地启动

先生成 protobuf 代码，再构建独立 HUB：

```sh
make generate-hub-proto
make build-hub
```

准备服务端证书和私钥，编辑 `hub/configs/hub.example.json` 中的路径，然后启动：

```sh
./bin/hub --config ./hub/configs/hub.example.json
```

健康检查目前使用 gRPC 标准 Health 服务。`instance_id` 应由部署环境稳定配置；HA 阶段会将其用于租约和会话归属。
