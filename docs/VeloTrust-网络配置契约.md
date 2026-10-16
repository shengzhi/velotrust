# VeloTrust 客户端与 Gateway 网络配置契约

本文定义当前静态配置版本中 Windows Client CLI 与 Linux Gateway Agent 的 WireGuard、地址、Endpoint、AllowedIPs、路由和 MTU 语义。配置仍由管理员本地维护；客户端注册、控制面签名配置下发和策略版本协商尚未实现。

## 字段语义

| 概念 | Client 配置 | Gateway 配置 | 约定 |
|---|---|---|---|
| 本地隧道地址 | `tunnel.address` | `tunnel.address` | 分别配置在本地 WireGuard 接口上的 CIDR。双方地址必须可路由且不冲突；Gateway 对客户端 Peer 的 `allowed_ips` 必须覆盖客户端地址。 |
| WireGuard 公钥 | `tunnel.peers[].public_key` 是 Gateway 公钥 | `tunnel.peers[].public_key` 是客户端或远端 Gateway 公钥 | 对端配置本节点公钥；本节点私钥只保存在本机。当前 WireGuard 密钥同时作为节点网络标识，没有第二套节点身份密钥。 |
| UDP Endpoint | 每个 `tunnel.peers[].endpoint` 必填 | 本机 `tunnel.listen_port` 必填；Peer 的 `endpoint` 可省略 | Endpoint 格式为 `host:port`，端口为 1–65535。客户端 Endpoint 指向 Gateway 可达的 UDP 地址和 `listen_port`。Gateway 可省略 Peer Endpoint，让 WireGuard 从收到的报文学习客户端地址；固定地址或网关互联时可填写。 |
| Peer `allowed_ips` | 必填 | 必填 | 指定该 WireGuard Peer 的加密选路前缀和可接受源地址范围；它不授予业务访问权。精确相同的前缀不能分配给同一节点上的多个 Peer；不同长度的重叠前缀允许，由最长前缀匹配选择 Peer。 |
| 操作系统路由 | 从各 Peer 的 `allowed_ips` 自动创建 | 顶层 `routes` 显式创建到 `tunnel.interface` 的路由 | Client 当前没有独立 `routes` 字段。Gateway 的每条 `routes` 必须落在至少一个 Peer `allowed_ips` 前缀内，保证系统路由选入隧道后 WireGuard 也能选出对应 Peer。路由存在不等于授权，转发仍由 ACL 默认拒绝并按规则放行。 |
| MTU | `tunnel.mtu` | `tunnel.mtu` | 单位为字节，合法范围均为 1280–9000。它表示本地隧道接口 MTU，不是 WireGuard Peer 协商字段。客户端省略时为 1380；Gateway 省略时为 1420，以保留旧 Gateway 行为。部署时应按两端外层路径 MTU 选择值；不确定时显式配置相同值，并按链路条件降低。 |
| 默认路由 | 暂不支持 `0.0.0.0/0` 或 `::/0` | 只有 `allow_default_route: true` 才接受 | 当前 Client CLI 仅支持分流路由，不应用全隧道默认路由。Gateway 的默认路由须显式 opt-in。 |

## 路由与访问控制

Client 将 Peer `allowed_ips` 汇总为本机路由，并将每个前缀留给对应 WireGuard Peer 做加密选路。精确重复前缀会在配置校验时拒绝，避免路由无法确定应使用哪个 Gateway。

Gateway 的 Peer `allowed_ips` 定义 WireGuard 层的远端地址范围。顶层 `routes` 是额外写入 Linux 路由表、出口为受管 WireGuard 接口的网络前缀；每条路由都必须由 Peer `allowed_ips` 覆盖。Gateway 校验器拒绝无法匹配任何 Peer 的路由和跨 Peer 重复的精确前缀。`allowed_ips` 与 `routes` 都不替代 `rules` ACL：未匹配允许规则的转发流量仍默认拒绝。

Gateway Peer 配置客户端地址时，应使用客户端分配地址对应的主机前缀，例如客户端为 `10.100.0.2/32` 时，Gateway Peer 的 `allowed_ips` 包含 `10.100.0.2/32`。客户端 Peer 则至少包含 Gateway 隧道地址主机前缀及获准资源网段。

## MTU 兼容性

Gateway 新增可选 `tunnel.mtu`。省略该字段的旧配置继续使用 1420；配置文件规范化和 Agent 热重载将其视为有效 MTU 1420。该值同时应用于内核 WireGuard 接口和 WireGuard-Go TUN 接口。Client 仍使用 1380 作为默认值，双方 MTU 不要求协议层完全相同，但任何一侧都不能超出实际外层路径承载能力。手动配置时建议显式设置两端的 MTU，避免依赖不同平台默认值。

## 版本与校验

- 当前 Client 本地配置和 Gateway 配置均使用 `schema_version: 1`；两种 JSON 结构不同，不能将一个服务的配置文件直接交给另一个服务。
- WireGuard 公钥采用 32 字节标准 Base64。客户端需配置 Gateway 公钥；Gateway Peer 需配置客户端公钥。
- Client Endpoint 必须填写；Gateway Peer Endpoint 可空。Endpoint 域名在连接/应用时解析，解析失败会阻止该配置启动或应用。
- CIDR 在比较和应用前掩码规范化。Gateway `routes` 必须被某个 Peer `allowed_ips` 包含；默认路由需 Gateway 明确启用，Client 当前拒绝默认路由。
- Gateway `tunnel.mtu` 与 Client `tunnel.mtu` 均只接受 1280–9000；0 或省略表示各自文档所述默认值。

## 示例

Gateway：

```json
{
  "schema_version": 1,
  "gateway_id": "gw-site-a",
  "generation": 1,
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
