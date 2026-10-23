# VeloTrust 客户端与 Gateway 网络配置契约

本文定义 Windows Client CLI 与 Linux Gateway Agent 配置的 schema，以及 WireGuard、地址、Endpoint、AllowedIPs、路由和 MTU 语义。客户端配置由管理员维护；Gateway 可使用本地配置或由 HUB 从 IPAM、Peer、网络站点及 NAT 业务记录生成数据面快照。配置摘要只用于完整性和收敛校验。

## 字段语义

| 概念 | Client 配置 | Gateway 配置 | 约定 |
|---|---|---|---|
| 隧道地址 | `tunnel.address` | `tunnel.address` | 配置在本地 WireGuard 接口上的 CIDR。双方地址必须可路由且不冲突；Gateway 对客户端 Peer 的 `allowed_ips` 必须覆盖客户端地址。Gateway 静态模式从本机配置读取；HUB 模式以快照下发值为准。 |
| WireGuard 公钥 | `tunnel.peers[].public_key` 是 Gateway 公钥 | `tunnel.peers[].public_key` 是客户端或远端 Gateway 公钥 | 对端配置本节点公钥；本节点私钥只保存在本机。当前 WireGuard 密钥同时作为节点网络标识，没有第二套节点身份密钥。 |
| UDP Endpoint | 每个 `tunnel.peers[].endpoint` 必填 | 本机 `tunnel.listen_port` 必填；Peer 的 `endpoint` 可省略 | Endpoint 格式为 `host:port`，端口为 1–65535。客户端 Endpoint 指向 Gateway 可达的 UDP 地址和 `listen_port`。Gateway 可省略 Peer Endpoint，让 WireGuard 从收到的报文学习客户端地址；固定地址或网关互联时可填写。 |
| Peer `allowed_ips` | 必填 | 必填 | 指定该 WireGuard Peer 的加密选路前缀和可接受源地址范围；它不授予业务访问权。精确相同的前缀不能分配给同一节点上的多个 Peer；不同长度的重叠前缀允许，由最长前缀匹配选择 Peer。 |
| 操作系统路由 | 从各 Peer 的 `allowed_ips` 自动创建 | Peer `allowed_ips` 安装到 WireGuard；顶层 `routes` 校验本机已有出口路由 | Gateway `routes` 表示其后方的网络站点。Agent 使用主机路由查询确定可用出口和下一跳，不为站点网段添加重复的主机路由，也不允许出口回到受管 WireGuard 隧道或回环接口。Peer `allowed_ips` 中的远端网段另行安装到 WireGuard 接口。路由存在不等于授权，转发仍由 ACL 默认拒绝并按规则放行。 |
| MTU | `tunnel.mtu` | `tunnel.mtu` | 单位为字节，合法范围均为 1280–9000。它表示本地隧道接口 MTU，不是 WireGuard Peer 协商字段。客户端省略时为 1380；Gateway 省略时为 1420，以保留旧 Gateway 行为。部署时应按两端外层路径 MTU 选择值；不确定时显式配置相同值，并按链路条件降低。 |
| 默认路由 | 暂不支持 `0.0.0.0/0` 或 `::/0` | 只有 `allow_default_route: true` 才接受 | 当前 Client CLI 仅支持分流路由，不应用全隧道默认路由。Gateway 的默认路由须显式 opt-in。 |

## Gateway 使用 HUB 同步时的配置归属

Gateway 本机配置固定操作系统相关的隧道接口名、WireGuard UDP 监听端口、MTU、私钥和 HUB 连接参数。快照中的隧道地址、Peer、关联站点网段、ACL 和 NAT 是数据面权威配置。HUB 快照省略 `tunnel.interface`、`tunnel.listen_port` 和 MTU；Agent 在应用前与本机配置合并。Gateway 对活动关联站点自动应用 Overlay 到站点网段的 SNAT/Masquerade；影子地址映射只增加 DNAT。当前 Overlay 仅支持 IPv4，HUB 会拒绝关联 IPv6 站点的配置，避免无 SNAT 状态下错误放行。

HUB 管理模式下，Client 的 `tunnel.address` 由 HUB 从活动 IPAM 租约生成。HUB 按设备所有者直接所属的活动 IAM 用户组匹配动态 `User-Device` 地址池的 `client_group_id` 并分配地址租约。该关联只决定 Overlay IP，不创建 Client Peer，也不选择 Gateway；Peer 和 Gateway 集合应由访问策略决定。Peer 地址必须与有效租约一致。

启用 HUB 时，本机配置可省略 `tunnel.address`，因为启动后的隧道地址由首个有效快照提供。在获得并成功应用快照前，Gateway 转发保持阻断；快照必须提供有效的 `tunnel.address`。未启用 HUB 的静态配置仍需在本机配置隧道地址、接口名和监听端口。

## 路由与访问控制

Client 将 Peer `allowed_ips` 汇总为本机路由，并将每个前缀留给对应 WireGuard Peer 做加密选路。精确重复前缀会在配置校验时拒绝，避免路由无法确定应使用哪个 Gateway。

Gateway 的 Peer `allowed_ips` 定义 WireGuard 层的远端地址范围，并由 Agent 生成经过隧道的系统路由。顶层 `routes` 来自绑定到 Gateway 的网络站点，表示本机应转发到站点 LAN 的目标网段；Agent 根据主机已有路由查询出口，不另行安装站点路由。Overlay 源地址在转发到活动关联站点时自动 Masquerade，返回包由连接跟踪还原并送回隧道客户端。若 Peer AllowedIPs 的更具体前缀覆盖了站点 CIDR 的一部分，配置校验会拒绝。`allowed_ips` 与 `routes` 都不替代 `rules` ACL：未匹配允许规则的转发流量仍默认拒绝。

Gateway Peer 配置客户端地址时，应使用客户端分配地址对应的主机前缀，例如客户端为 `10.100.0.2/32` 时，Gateway Peer 的 `allowed_ips` 包含 `10.100.0.2/32`。客户端 Peer 则至少包含 Gateway 隧道地址主机前缀及获准资源网段。

## MTU 兼容性

Gateway 新增可选 `tunnel.mtu`。省略该字段的旧配置继续使用 1420；配置文件规范化和 Agent 热重载将其视为有效 MTU 1420。该值同时应用于内核 WireGuard 接口和 WireGuard-Go TUN 接口。Client 仍使用 1380 作为默认值，双方 MTU 不要求协议层完全相同，但任何一侧都不能超出实际外层路径承载能力。手动配置时建议显式设置两端的 MTU，避免依赖不同平台默认值。

## 版本与校验

- 当前 Client 本地配置和 Gateway 配置均使用 `schema_version: 1`；两种 JSON 结构不同，不能将一个服务的配置文件直接交给另一个服务。
- WireGuard 公钥采用 32 字节标准 Base64。客户端需配置 Gateway 公钥；Gateway Peer 需配置客户端公钥。
- Client Endpoint 必须填写；Gateway Peer Endpoint 可空。Endpoint 域名在连接/应用时解析，解析失败会阻止该配置启动或应用。
- CIDR 在比较和应用前掩码规范化。Gateway Peer `allowed_ips` 的默认路由需 Gateway 明确启用；Client 当前拒绝默认路由。关联网络 routes 只接受非默认目的网段，出口下一跳可以来自主机默认路由。
- Gateway `tunnel.mtu` 与 Client `tunnel.mtu` 均只接受 1280–9000；0 或省略表示各自文档所述默认值。

## 示例

Gateway：

```json
{
  "schema_version": 1,
  "tunnel": {
    "interface": "vtrust0",
    "address": "10.100.0.1/24",
    "listen_port": 51820,
    "mtu": 1380,
    "private_key_file": "wireguard.key",
    "peers": [
      {
        "id": "windows-client",
        "public_key": "替换为客户端公钥",
        "allowed_ips": ["10.100.0.2/32"]
      }
    ]
  },
  "routes": [],
  "rules": []
}
```

Client 对应字段：

```json
{
  "tunnel": {
    "address": "10.100.0.2/32",
    "mtu": 1380,
    "private_key_file": "client.key",
    "peers": [
      {
        "id": "gateway-site-a",
        "public_key": "替换为 Gateway 公钥",
        "endpoint": "gateway.example.com:51820",
        "allowed_ips": ["10.100.0.1/32", "10.20.0.0/24"]
      }
    ]
  }
}
```

此处 Client 的 `allowed_ips` 会在 Windows 创建两条隧道路由；Gateway 的客户端 Peer `allowed_ips` 只声明客户端源地址。Gateway 的 `rules` 仍需显式允许 `10.100.0.2/32` 到 `10.20.0.0/24` 的所需协议和端口。
