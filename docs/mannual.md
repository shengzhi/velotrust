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
  "gateway_id": "gw-site-a",
  "generation": 1,
  "tunnel": {
    "interface": "vtrust0",
    "address": "10.100.0.1/24",
    "listen_port": 51820,
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

以前台方式启动：

```sh
sudo /usr/local/bin/gateway-agent run \
  --config /etc/velotrust/gateway/gateway.json \
  --health-listen 127.0.0.1:8765 \
  --log-level debug
```

Gateway Agent 会在 `listen_port` 上启动 WireGuard UDP 监听，并应用 Peer、路由和防火墙规则。需在 Gateway 主机防火墙及云安全组放行 UDP `51820`。运行用户还需要创建隧道、配置路由、nftables 和 IP 转发的权限。空 `rules` 表示默认拒绝转发。

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

`readyz` 表示配置已成功应用，不代表任何 Peer 已完成握手。修改配置时递增 `generation`，写入完整文件后再原子替换；运行中的 Agent 会监视配置变更并重新应用。

### 连接和转发条件

- 客户端 Peer 的 `public_key` 必须是客户端 WireGuard 公钥，`allowed_ips` 至少包含该客户端的隧道地址，例如 `10.100.0.2/32`。
- Gateway 对外 Endpoint 使用 Gateway 主机可达地址和监听端口，例如 `gateway.example.com:51820`。
- 若资源位于 Gateway 直连的 LAN，优先确保 LAN 有返回隧道网段的路由；无法配置回程路由时，可为指定流量配置 SNAT。经其他隧道可达的网段则需配置对应路由和 Peer。
- `rules` 必须显式允许需要的流量；只配置 WireGuard Peer 不会自动授权访问资源。

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

修改配置时递增 `generation`，再校验并重启或等待 Agent 热重载：

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

将更新后的 Gateway 配置递增 `generation` 并应用后，在 `$ClientDir\client.json` 创建客户端连接配置：

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
        "allowed_ips": ["10.100.0.1/32", "10.20.0.0/24"],
        "persistent_keepalive": 25
      },
      {
        "id": "gateway-site-b",
        "public_key": "替换为站点 B Gateway WireGuard 公钥",
        "endpoint": "gateway-b.example.com:51820",
        "allowed_ips": ["10.100.0.3/32", "10.30.0.0/24"],
        "persistent_keepalive": 25
      }
    ]
  }
}
```

`peers` 中每项对应一个 Gateway，`allowed_ips` 填入该 Gateway 的隧道地址和资源网段，不要填默认路由。此例中客户端用同一个 WireGuard 公钥分别登记在两个 Gateway 上；每个 Gateway 仍独立执行自己的访问规则。各 peer 的 endpoint 不能被任何 `allowed_ips` 覆盖，否则客户端会拒绝连接以避免 UDP 流量递归进入隧道。不要把相同的精确网段分配给多个 Gateway；若远端网段冲突，为各 Gateway 配置不同的 NAT 映射网段。当前支持同时访问不同网段，尚不提供同一网段的主备故障切换。旧版单 Gateway 配置的 `peer` 字段仍兼容，新配置请使用 `peers` 数组。校验配置：

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

CLI 会创建一个 Wintun 接口、应用隧道地址和聚合后的路由，并持续运行。`--log-level debug` 会逐 Gateway 输出 Endpoint、公钥指纹、Peer 是否存在、握手时间和收发字节数，不会输出私钥。每个 Gateway 握手成功时会分别显示 `WireGuard handshake established with gateway "<id>".`。随后可在另一个窗口测试获准资源：

连接启动时会先打印 `Client WireGuard public key`。将该公钥加入每个允许该客户端连接的 Gateway 配置中的 `tunnel.peers[].public_key`，并确保对应的 `allowed_ips` 包含客户端隧道地址。Gateway 与 Client 打印的都是 WireGuard 公钥；不要交换或粘贴私钥文件。

```powershell
Test-NetConnection 10.20.0.10 -Port 443
```

按 Ctrl+C 正常断开并清理本次配置。如果进程异常退出，在没有其他连接进程运行时执行：

```powershell
& $Exe disconnect --config $ClientConfig
```

### 常见连接问题

- **没有握手**：确认 Gateway 正在运行、UDP 监听端口已从客户端网络放通、两端公钥和 Peer 配置匹配、Endpoint 地址正确。
- **握手成功但资源不可达**：检查 Gateway 的 `rules`、资源路由和返回客户端隧道网段的路由；确认目标端口与规则一致。
- **连接或清理提示权限不足**：使用提升权限的 PowerShell 重试，并确认 `wintun.dll` 与 CLI 位于同一目录。

## 当前边界

客户端连接使用本地静态配置，尚未实现用户登录、设备注册、控制面授权配置同步、Windows 后台服务、DNS 管理和休眠恢复。Gateway Agent 尚未连接管理控制面。真实 Windows 与 Gateway 环境的端到端握手和资源访问仍需在目标网络中验证。
