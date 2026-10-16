# signal

面向终端和网关 Agent 的南向 gRPC 接入服务，管理双向流会话、状态上报和控制指令传递。Protobuf 源定义统一维护在 `../shared/proto`。

- `../cmd/signal`：按需独立部署时的服务入口。
- `internal/auth`、`internal/session`：节点认证与会话生命周期。
- `internal/stream`、`internal/dispatch`：双向流处理与指令分发。
- `internal/report`：状态和运行结果接收。
