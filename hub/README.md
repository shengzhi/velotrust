# HUB

HUB 是运行控制平面，负责节点认证、注册凭证校验、配置与策略同步、会话管理、Peer 状态及运行状态上报。实际业务流量由 Gateway 和 Client Agent 直接通过隧道传输，不经过 HUB。

## 当前代码框架

- `config/`：HUB 监听地址、稳定实例标识、TLS、PostgreSQL 和心跳参数。
- `app/`：HUB 组合根；独立进程和未来合并部署复用同一套服务装配。
- `runtime/`：服务生命周期和优雅关闭，可由独立入口或后续 `cmd/server` 组合启动。
- `transport/grpc/`：gRPC 服务端、TLS 与标准健康检查。
- `internal/auth/`：Gateway 当前节点身份校验。
- `internal/session/`：单实例内存会话租约；多实例共享租约尚未实现。
- `internal/repository/postgres/`：节点目录、心跳和运行状态访问。
- `internal/model/`：HUB 按需使用的数据库持久化模型。
- `internal/configsync/`：节点期望配置读取接口及独立于 Management 数据实体的配置快照。
- `internal/report/`：节点运行状态上报接口。
- `shared/proto/v1/hub.proto`：跨进程控制通道契约；生成代码位于 `shared/gen/go/hub/v1/`。

当前已接入 PostgreSQL Gateway 节点目录校验和 Gateway X25519 控制流认证：新建 gRPC 流先发送 Hello，HUB 核验节点处于 active 状态且公钥与数据库绑定一致，再发出一次性挑战；Gateway 使用注册的 WireGuard X25519 私钥及 TLS exporter（TLS 模式）或显式明文挑战上下文（明文模式）计算证明。认证成功后服务端才发送 Welcome。认证后的心跳会续租单实例内存会话，并更新 `t_ztna_node.last_heartbeat_at`；运行状态写入节点当前状态及 `t_ztna_node_status_report`，节点收到状态确认后才知道该报告已持久化。

Gateway 首次注册由 `gateway-agent run` 自动处理：Agent 先执行常规身份认证，HUB 仅对尚未绑定公钥的预建节点返回专用“未注册”错误；Agent 随后使用配置中的一次性凭证完成注册专用 X25519 密钥持有证明。HUB 在单个 PostgreSQL 事务中消费凭证并绑定公钥。若需要 Management 审批，HUB 先返回待审批状态并保持注册 gRPC 流，定期读取节点状态；节点获批后再返回最终结果。Agent 收到通过结果后重新执行常规身份认证，认证成功前转发保持阻断。凭证无效、过期、撤销、重复使用、目标节点已注册或公钥冲突均拒绝。Gateway 配置同步由 HUB 从 Gateway 节点的 IPAM 租约、活动隧道 Peer、关联站点网络及 NAT 台账编译期望快照；计算摘要后通过欢迎消息或运行状态应答发送。Gateway 校验内容摘要和本机配置边界后调用 Reconciler 替换隧道、ACL、防火墙、路由及 NAT 状态，完成后回执。HUB 将发送与回执写入配置任务和事件，运行状态持续报告实际配置摘要；普通网络中断期间 Gateway 保留最近一次已验证配置；重连后重新认证并对账。无 IPAM 租约、业务数据无效或读取失败时，Gateway fail-closed。配置变化通过 Gateway 心跳周期内的状态上报发现。Client 身份认证、跨实例会话协调、撤销处理及 HA 尚未实现。Gateway Agent 报告反映本地 Reconciler 状态；Peer 数当前标记为未知。X25519 通道绑定协议仍须完成独立安全评审和真实客户端互通验证后再用于生产。

### Gateway 当前配置格式与安全边界

HUB 将编译后的数据面 JSON 作为协议快照发送，包含 `schema_version`、IPAM `tunnel.address`、活动 Peer、关联站点网段 `routes`、受支持的 NAT 映射和 `rules`。`tunnel.interface`、`tunnel.listen_port`、MTU、WireGuard 私钥及 HUB 连接设置只由 Gateway 本机配置控制；快照省略这些字段，Agent 在应用前与本机配置合并。`routes` 是 Gateway 本地应转发到的目的网络，Agent 检查它必须有非默认本机路由，并且不能经由 WireGuard 接口；Agent 不会把这些站点网段添加成 WireGuard 出口路由。策略规则目前没有稳定的数据库编译格式，因此 `rules` 为空，按默认拒绝处理。`config_hash` 是编译 JSON 的 canonical SHA-256 十六进制摘要，存入配置任务及节点状态，用于对账；不保存配置 JSON 副本。

Agent 启动时保持转发阻断，直到通过 HUB 身份认证并成功应用首个快照。相同配置摘要会幂等确认；旧回执不能覆盖新的当前配置。普通网络中断或 HUB 暂时不可用时，Gateway 保留最近一次已验证配置；节点重连后必须重新认证并按数据库当前状态对账。节点授权失效、快照损坏、摘要不匹配、配置应用失败或已应用配置从数据库移除时，Gateway 会 fail-closed。

## 协议选择

HUB 南向控制通道采用 **gRPC over HTTP/2 + Protocol Buffers**。节点连接后使用双向流承载心跳、配置下发、应用回执和状态上报；注册、诊断等请求使用 unary RPC。TLS 是可选项：HUB 配置 `tls.enabled: false` 或省略 TLS 配置可运行明文 gRPC；Gateway 端的 `hub.tls` 默认启用，因此连接明文 HUB 时也必须显式设置为 `false`。两端传输模式必须一致。明文模式只保留 X25519 节点密钥持有证明，不提供传输加密、消息完整性或 HUB 服务端身份认证，只适用于隔离的开发网络。生产部署应启用 TLS。

明文开发环境的 HUB 配置可将 TLS 关闭：

```json
"tls": { "enabled": false }
```

Gateway 配置也要显式关闭 TLS：

```json
"hub": { "endpoint": "127.0.0.1:8443", "tls": false }
```

Go 标准库 `net/rpc` 不作为候选：它是已冻结的通用 RPC 包，缺少本项目需要的双向流和跨语言协议生成能力。自定义 TCP 会要求项目自行维护消息 framing、版本兼容、重连、背压、流控、超时及认证集成；除非未来有明确的 HTTP/2 不可用环境或性能证据，否则不承担这套协议维护成本。

## 本地启动

先生成 protobuf 代码，再构建独立 HUB：

```sh
make generate-hub-proto
make build-hub
```

准备服务端证书和私钥，编辑 `hub/configs/hub.example.json` 中的路径，然后启动：

```sh
export DATABASE_DSN='postgres://velotrust:change-me@127.0.0.1:5432/velotrust?sslmode=disable'
./bin/hub --config ./hub/configs/hub.example.json
```

数据库需先应用当前 PostgreSQL ZTNA schema/migrations。HUB 启动时验证数据库连通性。健康检查目前使用 gRPC 标准 Health 服务；数据库运行中断后的依赖健康状态尚未接入。`heartbeat_timeout_seconds` 控制单实例会话租约和断连判定阈值；当前在线状态未提供 Management 查询接口。`instance_id` 应由部署环境稳定配置，HA 阶段会将其用于共享租约和会话归属。


#### 配置如何更新

配置通过业务数据维护，不再手工提交 Gateway JSON。为 Gateway 分配 IPAM 租约、调整其关联站点、隧道 Peer 或 NAT 台账后，HUB 在节点首次认证或下一次状态对账时重新编译；内容摘要变化时创建 `apply_config` 任务。节点详情中的“最近下发配置摘要”和“节点回执配置摘要”用于检查收敛；状态迁移记录于 `t_ztna_node_config_task_event`。策略 `rules_json` 当前没有 Gateway 规则编译器，故发往 Gateway 的 ACL 仍为空并 fail-closed。
