# HUB

HUB 是运行控制平面，负责 Gateway 首次注册凭证校验、Client 员工账号登录和设备登记、节点认证、配置与策略同步、会话管理、Peer 状态及运行状态上报。实际业务流量由 Gateway 和 Client Agent 直接通过隧道传输，不经过 HUB。

## 当前代码框架

- `config/`：HUB 监听地址、稳定实例标识、TLS、PostgreSQL 和心跳参数。
- `app/`：HUB 组合根；独立进程和未来合并部署复用同一套服务装配。
- `runtime/`：服务生命周期和优雅关闭，可由独立入口或后续 `cmd/server` 组合启动。
- `transport/grpc/`：gRPC 服务端、TLS 与标准健康检查。
- `internal/auth/`：Gateway 与 Client 节点身份校验。
- `internal/session/`：单实例内存会话租约；多实例共享租约尚未实现。
- `internal/repository/postgres/`：节点目录、心跳和运行状态访问。
- `internal/model/`：HUB 按需使用的数据库持久化模型。
- `internal/configsync/`：节点期望配置读取接口及独立于 Management 数据实体的配置快照。
- `internal/report/`：节点运行状态上报接口。
- `shared/proto/v1/hub.proto`：跨进程控制通道契约；生成代码位于 `shared/gen/go/hub/v1/`。

Gateway 和 Client 均通过 PostgreSQL 节点目录校验及 X25519 挑战完成控制流认证：新建 gRPC 流先发送 Hello，HUB 核验节点状态、公钥及关联凭证，再发出一次性挑战；节点使用自己的 WireGuard X25519 私钥及 TLS exporter（TLS 模式）或明文挑战上下文（明文模式）计算证明。认证成功后服务端才发送 Welcome。Client 的目录校验还要求关联设备、所属用户和设备公钥凭证都处于有效状态。认证后的心跳续租单实例内存会话；运行状态写入节点当前状态及 `t_ztna_node_status_report`，节点收到状态确认后才知道该报告已持久化。

Gateway 首次注册由 `gateway-agent run` 使用 Management 签发的一次性凭证自动处理。Client 员工账号由 Management「员工用户管理」页面创建；员工在 Client Agent 登录时输入账号密码，Client 只把凭证发送给启用 TLS 的 HUB，不调用 Management API。HUB 校验员工账号后，在单个 PostgreSQL 事务中首次创建 Client 节点、设备及其 X25519 公钥凭证；之后每次 Client 连接都校验员工密码和账号状态，并要求设备私钥完成 X25519 挑战应答。密码错误、账号禁用、设备禁用或公钥与已绑定设备不一致时拒绝连接。HUB 根据员工所属的活动安全用户组与 Gateway 准入绑定编译 Client Peer 配置，不依赖逻辑隧道成员关系；Gateway 必须处于活动状态，并具有 WireGuard 公钥、有效 IPAM 隧道地址和可用公网端点（节点公网端点优先，其次使用 Gateway IPAM 租约端点）。内容摘要通过欢迎消息或运行状态应答下发，Client 校验摘要、创建 Windows WireGuard 隧道并回执。禁用节点、设备或所属员工会使认证失败，并通过活动流通知在线 Client 清理本机隧道；离线 Client 会在下次认证时被拒绝。Client 的管理控制只连接 HUB，不调用 Management API。

HUB 的 Client 与 Gateway 配置同步均在节点重连及心跳状态上报时对账。普通临时断连期间 Client 保留最近应用的隧道；收到访问撤销通知、授权失效或配置应用失败时 Client 停止本机隧道。当前会话租约仍为单实例内存状态，跨 HUB 实例会话协调、Client 配置回执的独立持久化和 HA 故障恢复尚未实现。Gateway Agent 报告反映本地 Reconciler 状态；Peer 数当前标记为未知。X25519 通道绑定协议仍须完成独立安全评审和真实客户端互通验证后再用于生产。

### Gateway 当前配置格式与安全边界

HUB 将编译后的数据面 JSON 作为协议快照发送，包含 `schema_version`、IPAM `tunnel.address`、活动 Peer、关联站点网段 `routes`、受支持的 NAT 映射和 `rules`。`tunnel.interface`、`tunnel.listen_port`、MTU、WireGuard 私钥及 HUB 连接设置只由 Gateway 本机配置控制；快照省略这些字段，Agent 在应用前与本机配置合并。`routes` 是 Gateway 本地应转发到的目的网络，Agent 检查它必须有非默认本机路由，并且不能经由 WireGuard 接口；Agent 不会把这些站点网段添加成 WireGuard 出口路由。活动策略的 `rules_json` 由 Management 保存为当前状态，规则按目标 Gateway 编译为 ACL；策略优先级和规则优先级决定先后次序，未匹配 allow 的流量由 Gateway 默认拒绝。`config_hash` 是编译 JSON 的 canonical SHA-256 十六进制摘要，存入配置任务及节点状态，用于对账；不保存配置 JSON 副本。

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

Client 对应字段为 `hub.tls_enabled: false`。当且仅当 HUB 也显式关闭 TLS 时，HUB 才接受明文员工密码登录。员工密码会以明文通过网络传输；此模式仅用于隔离的开发网络，生产部署必须启用 TLS。

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

配置通过业务数据维护，不再手工提交 Gateway JSON。为 Gateway 分配 IPAM 租约、调整其关联站点、隧道 Peer、NAT 台账、企业资源或 Management 访问策略后，HUB 在节点首次认证或下一次状态对账时重新编译。访问策略关联安全用户组和企业资源；HUB 根据组成员设备的活动 Overlay 租约解析 ACL 来源，根据资源记录、站点 NAT 映射及站点 Gateway 解析 ACL 目标、协议和端口，并同步 Client Peer/路由与 Gateway ACL。策略、资源、用户组或租约变化会写入访问变更 outbox 并通知相关 Client/Gateway。内容摘要变化时创建 `apply_config` 任务。节点详情中的“最近下发配置摘要”和“节点回执配置摘要”用于检查收敛；没有匹配的活动 allow 规则时，Gateway 默认拒绝转发。
