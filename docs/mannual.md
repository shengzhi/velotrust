# VeloTrust 服务操作手册

本文按服务说明部署和验证 Gateway Agent 与 Windows Client CLI。示例使用：Gateway 隧道地址 `10.100.0.1/24`、客户端地址 `10.100.0.2/32`、资源网段 `10.20.0.0/24`、WireGuard UDP 端口 `51820`。请按实际网络替换示例值。

## Gateway Agent（Linux）

### 配置并启动

在 Gateway 主机准备好 `gateway-agent` 程序后，生成唯一的 WireGuard 节点密钥。该 WireGuard 公钥同时作为节点的网络身份标识。私钥目录必须位于 Linux 本地文件系统，并限制为当前运行用户可访问：

```sh
sudo install -d -m 700 /etc/velotrust/gateway
sudo /usr/local/bin/gateway-agent keygen --dir /etc/velotrust/gateway
```

将客户端公钥加入 `tunnel.peers`，并按需要配置允许访问的资源规则。以下配置示例只允许该客户端访问资源网段的 TCP 443：

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
        "id": "windows-test-client",
        "public_key": "替换为客户端公钥",
        "allowed_ips": ["10.100.0.2/32"]
      }
    ]
  },
  "rules": [
    {
      "id": "allow-windows-test-https",
      "priority": 100,
      "source_prefix": "10.100.0.2/32",
      "destination_prefix": "10.20.0.0/24",
      "protocol": "tcp",
      "port_from": 443,
      "port_to": 443,
      "action": "allow"
    }
  ]
}
```

保存为 `/etc/velotrust/gateway/gateway.json`，限制配置文件权限并校验：

```sh
sudo chmod 600 /etc/velotrust/gateway/gateway.json
sudo /usr/local/bin/gateway-agent validate --config /etc/velotrust/gateway/gateway.json
```

配置文件应显示为 `600` 权限。若在 WSL 中配置位于 `/mnt/c` 等 Windows 挂载目录，且 `chmod` 后权限仍不变，请将 `gateway.json` 和它引用的私钥一并移到 WSL Linux 文件系统（例如 `/home/<user>/.config/velotrust/gateway/`），再对配置和私钥执行 `chmod 600`。

首次接入 HUB 时，在配置文件中添加 `hub.endpoint`、`hub.server_name`、可选的 `hub.ca_file`，以及 Management 为预建网关签发的一次性 `hub.registration_token`。配置中不需要填写 `gateway_id`：首次注册时 HUB 通过 Token 定位预建节点，注册后通过本机 WireGuard 公钥定位节点。配置文件必须保持 `0600` 权限。启动 `run` 后，Agent 先尝试 X25519 身份认证；HUB 明确返回该公钥尚未注册时，Agent 才提交 Token。若需要审批，Agent 会保持运行并等待 HUB 返回审批结果，期间转发保持阻断；获批后 Agent 会重新认证，再应用隧道和规则。升级 Agent 前应先升级 HUB，使 HUB 返回包含规范节点编码的新认证挑战。

注册完成后，从配置文件删除已消费的 `hub.registration_token`。程序会清除活动配置字段及注册客户端的临时凭证缓冲，但不会改写配置文件。已注册节点无需配置 Token。使用 HUB 管理配置时，还要在本机配置中加入 HUB 地址及 TLS 信任设置，例如：

```json
{
  "schema_version": 1,
  "hub": {
    "endpoint": "hub.example.com:8081",
    "server_name": "hub.example.com",
    "ca_file": "hub-ca.pem"
  },
  "tunnel": {
    "interface": "vtrust0",
    "listen_port": 51820,
    "private_key_file": "wireguard.key"
  }
}
```

HUB 模式下，本机配置只负责隧道接口名、WireGuard UDP 监听端口、私钥和 HUB 连接参数；隧道地址、Peer、关联站点网段和 NAT 由 HUB 从 IPAM、隧道、网络站点及 NAT 业务数据编译。Gateway 必须先分配 IPAM 隧道地址，且关联站点网络应有本机物理或静态路由。策略规则尚未接入 Gateway 编译，当前 ACL 默认拒绝。首个有效快照应用前转发保持阻断。业务数据修改后，HUB 在 Gateway 心跳或重连时重新编译并发送摘要不同的配置。

以前台方式启动：

```sh
sudo /usr/local/bin/gateway-agent run \
  --config /etc/velotrust/gateway/gateway.json \
  --health-listen 127.0.0.1:8765 \
  --log-level debug
```

Gateway Agent 会在 `listen_port` 上启动 WireGuard UDP 监听，并应用 Peer、路由和防火墙规则。需在 Gateway 主机防火墙及云安全组放行 UDP `51820`。运行用户还需要创建隧道、配置路由、nftables 和 IP 转发的权限。空 `rules` 表示默认拒绝转发。

`tunnel.mtu` 以字节为单位，省略时 Gateway 使用 1420；客户端省略时使用 1380。HUB 快照省略 MTU 时 Gateway 保留本机配置值。关联网络 `routes` 是 Gateway 后方的本地目的网段；Gateway 根据主机当前路由选择出口接口和下一跳，并安装受管站点路由。出口不能是 VeloTrust 隧道或回环接口；系统默认路由可以提供下一跳。Peer 的 `allowed_ips` 则描述该 Peer 对应的隧道地址和远端路由网段。

启动日志会打印 `gateway WireGuard public key`。将该公钥填入客户端对应 Gateway 的 `tunnel.peers[].public_key`。

较早版本可能还生成了 `node-identity.key` 和 `node-identity.pub`；当前版本不读取它们，也不会自动删除。确认没有其他程序依赖后，可自行安全删除这两个旧文件。

`--log-level debug` 会将配置应用阶段、Peer 公钥指纹、路由/防火墙阶段和内核 WireGuard 后端的隧道握手统计输出到标准错误；日志不包含私钥。用户态回退后端会记录配置状态，但不会通过调试接口读取含私钥的状态数据。

Linux Gateway Agent 会检查系统的 `iptables`/`ip6tables` 转发链。检测到 Docker 的 `DOCKER-FORWARD`（旧版 `DOCKER`）和已接入 `FORWARD` 的 `DOCKER-USER` 时，会创建按隧道接口隔离的 `VTGW-*` 链，并在 `DOCKER-USER` 的现有规则之后、首个 `RETURN` 之前挂接。链中只镜像当前 ACL 的允许流量和已建立回包；VeloTrust 自己的 nftables ACL 仍负责授权。Agent 不修改 Docker 的 `FORWARD` 默认策略。Agent 启动及 ACL 更新时会同步规则，运行期间每 15 秒重查一次，以便 Docker 重建链后自动恢复挂接；阻断转发、清空 ACL 或正常停止时会移除这组放行规则。

该集成要求 Agent 与 Docker 使用同一套 iptables 后端（iptables-nft 或 iptables-legacy），并且 Docker 的 `DOCKER-USER` 链已从 `FORWARD` 接入。可用以下命令查看实际规则；Agent 的 debug 日志会记录管理链名称：

```sh
sudo iptables -S DOCKER-USER
sudo ip6tables -S DOCKER-USER
```

如果主机使用没有 `DOCKER-USER` 链的原生 nftables 转发配置，Agent 不会猜测或改写其规则；需要由主机防火墙为 Gateway 隧道接口配置等价的精确放行规则。

另开终端检查服务健康状态：

```sh
curl http://127.0.0.1:8765/healthz
curl http://127.0.0.1:8765/readyz
```

`readyz` 表示配置已成功应用，不代表任何 Peer 已完成握手。修改配置后先校验，再将完整文件原子替换到配置路径；运行中的 Agent 会根据内容变化自动重新应用。

### 连接和转发条件

- 客户端 Peer 的 `public_key` 必须是客户端 WireGuard 公钥，`allowed_ips` 至少包含该客户端的隧道地址，例如 `10.100.0.2/32`。
- Gateway 对外 Endpoint 使用 Gateway 主机可达地址和监听端口，例如 `gateway.example.com:51820`。
- Client `allowed_ips` 生成 Windows 隧道路由；Gateway Peer `allowed_ips` 生成 Linux WireGuard 路由。Gateway 顶层 `routes` 是其后方的站点网段，Agent 按主机路由查找结果安装到物理出口，不能经 WireGuard 或回环接口；系统默认路由可以提供下一跳。
- 若资源位于 Gateway 直连的 LAN，优先确保 LAN 有返回隧道网段的路由；无法配置回程路由时，可为指定流量配置 SNAT。经其他 Gateway 可达的网段应放在对应 Peer 的 `allowed_ips` 中，由 Agent 安装到 WireGuard 接口。
- `rules` 必须显式允许需要的流量；只配置 WireGuard Peer 不会自动授权访问资源。

Client 与 Gateway 的字段定义、Endpoint 角色、AllowedIPs、路由与 MTU 约定见[网络配置契约](VeloTrust-网络配置契约.md)。

#### 为 LAN 访问配置 SNAT

当 LAN 主机没有返回客户端隧道网段的路由时，可以让 Gateway 使用其 LAN 地址作为客户端流量的来源地址。先确认连接 LAN 的接口和地址：

```sh
ip -br address
ip route get 10.20.0.10
```

在 Gateway 配置的 `nat` 数组中增加一条 SNAT 映射。以下示例假设隧道客户端使用 `10.100.0.0/24`，LAN 资源使用 `10.20.0.0/24`，`vtg0` 是实际出口接口，且 `10.20.0.1` 已配置在 `vtg0` 上：

```json
"nat": [
  {
    "type": "snat",
    "source_prefix": "10.100.0.0/24",
    "destination_prefix": "10.20.0.0/24",
    "egress_interface": "vtg0",
    "to_address": "10.20.0.1"
  }
]
```

`source_prefix`、`destination_prefix` 和出口接口共同限定转换范围；`to_address` 必须是配置在该出口接口上的单个地址。若同一流量还经过 DNAT，`destination_prefix` 应填写 DNAT 后的真实目标网段。SNAT 不会授予访问权限，仍需由 `rules` 显式允许原始客户端地址到 LAN 资源的流量。ACL 在转换前匹配原始源地址。

修改配置后校验并重启或等待 Agent 热重载：

```sh
sudo gateway-agent validate --config /etc/velotrust/gateway/gateway.json
sudo nft list chain inet velotrust_gateway_nat gateway_postrouting
```

规则应显示隧道入口、指定出口、源/目标前缀和 `snat to` 地址。可在出口接口抓包确认 LAN 主机看到的来源地址是 `to_address`；回包应经 Gateway 的连接跟踪还原后进入 WireGuard 隧道。若某个 LAN 主机已有正确的隧道回程路由，则不需要对此流量配置 SNAT。

## Windows Client CLI

### 运行前准备

将已部署的 `client-cli-windows-amd64.exe` 和 x64 `wintun.dll` 放在同一目录。生成密钥时使用普通 PowerShell；连接和断开隧道时使用**以管理员身份运行**、且与生成密钥相同 Windows 用户的 PowerShell。

```powershell
$Exe = "C:\VeloTrust\Client\client-cli-windows-amd64.exe"
& $Exe version
```

Client CLI 以当前进程前台运行，当前不是 Windows 后台服务。

### 生成客户端密钥

```powershell
$ClientDir = Join-Path $env:LOCALAPPDATA "VeloTrust\Client"
& $Exe keygen --dir $ClientDir
```

将命令输出的客户端**公钥**交给 Gateway 管理员，填入 Gateway 配置中的 Peer。私钥保存在 `$ClientDir\client.key`，不要复制或发送该文件。

将更新后的 Gateway 配置应用后，在 `$ClientDir\client.json` 创建客户端连接配置：

```json
{
  "schema_version": 1,
  "instance_id": "example-enterprise",
  "tunnel": {
    "interface_name": "VeloTrust",
    "address": "10.100.0.2/32",
    "private_key_file": "client.key",
    "mtu": 1380,
    "peers": [
      {
        "id": "gateway-site-a",
        "public_key": "替换为站点 A Gateway WireGuard 公钥",
        "endpoint": "gateway-a.example.com:51820",
        "allowed_ips": ["10.100.0.1/32", "10.20.0.0/24", "10.100.0.53/32", "10.100.0.54/32"],
        "persistent_keepalive": 25
      },
      {
        "id": "gateway-site-b",
        "public_key": "替换为站点 B Gateway WireGuard 公钥",
        "endpoint": "gateway-b.example.com:51820",
        "allowed_ips": ["10.100.0.3/32", "10.30.0.0/24"],
        "persistent_keepalive": 25
      }
    ],
    "dns": {
      "enabled": true,
      "resolver_groups": [
        {
          "domains": ["corp.example.com", "internal.example.com"],
          "resolvers": ["10.100.0.53:53"]
        },
        {
          "domains": ["dev.internal.example.com"],
          "resolvers": ["10.100.0.54:53"]
        }
      ]
    }
  }
}
```

`peers` 中每项对应一个 Gateway，`allowed_ips` 填入该 Gateway 的隧道地址和资源网段，不要填默认路由。此例中客户端用同一个 WireGuard 公钥分别登记在两个 Gateway 上；每个 Gateway 仍独立执行自己的访问规则。各 peer 的 endpoint 不能被任何 `allowed_ips` 覆盖，否则客户端会拒绝连接以避免 UDP 流量递归进入隧道。不要把相同的精确网段分配给多个 Gateway；若远端网段冲突，为各 Gateway 配置不同的 NAT 映射网段。当前支持同时访问不同网段，尚不提供同一网段的主备故障切换。旧版单 Gateway 配置的 `peer` 字段仍兼容，新配置请使用 `peers` 数组。校验配置：

DNS 示例中，`resolvers` 是企业 DNS 的 `IP:port` 地址，每个地址必须落在某个 Gateway Peer 的 `allowed_ips` 中，例如把 `10.100.0.53/32` 加入对应 Peer。域名按标签边界匹配，重叠后缀采用最长匹配。Windows Client 会把当前活动网络的系统 DNS 用作外部域名上游；内部域名只发给对应企业 DNS，失败时返回 `SERVFAIL`，不会回退到系统 DNS。

```powershell
$ClientConfig = Join-Path $ClientDir "client.json"
& $Exe config validate --file $ClientConfig --json
```

### 启动、检查和断开连接

在提升权限的 PowerShell 中启动：

```powershell
$Exe = "C:\VeloTrust\Client\client-cli-windows-amd64.exe"
$ClientConfig = Join-Path $env:LOCALAPPDATA "VeloTrust\Client\client.json"
& $Exe connect --config $ClientConfig --log-level debug
```

CLI 会创建一个 Wintun 接口、应用隧道地址和聚合后的路由，并持续运行。启用 DNS 时，Client 会启动仅监听回环地址的 UDP/TCP 53 本地代理，并通过 Windows NRPT 将系统 DNS 查询导入代理；启动前会检查端口并用一次系统 DNS 查询验证接管。`--log-level debug` 会逐 Gateway 输出 Endpoint、公钥指纹、Peer 是否存在、握手时间和收发字节数，也会显示 DNS 查询选择的上游类别，不会输出私钥。每个 Gateway 握手成功时会分别显示 `WireGuard handshake established with gateway "<id>".`。随后可在另一个窗口测试获准资源：

连接启动时会先打印 `Client WireGuard public key`。将该公钥加入每个允许该客户端连接的 Gateway 配置中的 `tunnel.peers[].public_key`，并确保对应的 `allowed_ips` 包含客户端隧道地址。Gateway 与 Client 打印的都是 WireGuard 公钥；不要交换或粘贴私钥文件。

```powershell
Test-NetConnection 10.20.0.10 -Port 443
```

使用系统 DNS 客户端验证域名分流：

```powershell
Resolve-DnsName app.corp.example.com -DnsOnly
Resolve-DnsName www.example.com -DnsOnly
```

在客户端 debug 日志中，内部域名应显示 `route=enterprise`，外部域名应显示 `route=original`。企业 DNS 地址必须经隧道路由可达，并由 Gateway 允许 UDP/TCP 53。可用 `Get-DnsClientNrptPolicy -Effective -Namespace "."` 查看全局 DNS 接管规则。

按 Ctrl+C 正常断开并移除本实例的 DNS 接管规则、路由和隧道地址。如果进程被强制结束，Windows 会暂时保留指向本机代理的 DNS 规则；在没有其他连接进程运行时，先执行下面的恢复命令：

```powershell
& $Exe disconnect --config $ClientConfig
```

### 常见连接问题

- **没有握手**：确认 Gateway 正在运行、UDP 监听端口已从客户端网络放通、两端公钥和 Peer 配置匹配、Endpoint 地址正确。
- **握手成功但资源不可达**：检查 Gateway 的 `rules`、资源路由和返回客户端隧道网段的路由；确认目标端口与规则一致。
- **连接或清理提示权限不足**：使用提升权限的 PowerShell 重试，并确认 `wintun.dll` 与 CLI 位于同一目录。

## 当前边界

客户端使用本地静态 DNS 策略，尚未接入控制面策略下发和应用状态回报。DNS 接管通过 Windows DNS Client/NRPT 工作；自行发送 DNS、DoH 和 DoT 的应用不在本阶段拦截范围内。macOS/Linux DNS 接管、Windows 网络切换时的真实端到端验证、用户登录、设备注册、Windows 后台服务和休眠恢复尚未实现。Gateway Agent 已可连接 HUB 并接收由 IPAM、隧道 Peer、网络站点和 NAT 业务数据编译的配置；访问策略尚未编译为 Gateway ACL，因此数据面继续默认拒绝。
