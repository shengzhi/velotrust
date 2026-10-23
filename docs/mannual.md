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

首次接入 HUB 时，在配置文件中添加 `hub.endpoint`、`hub.server_name` 和可选的 `hub.ca_file`。Gateway 首次注册还需要配置 Management 为预建网关签发的一次性 `hub.registration_token`。配置中不需要填写 `gateway_id`：首次注册时 HUB 通过 Token 定位预建节点，注册后通过本机 WireGuard 公钥定位节点。配置文件必须保持 `0600` 权限。启动 `run` 后，Agent 先尝试 X25519 身份认证；HUB 明确返回该公钥尚未注册时，Agent 才提交 Token。若需要审批，Agent 会保持运行并等待 HUB 返回审批结果，期间转发保持阻断；获批后 Agent 会重新认证，再应用隧道和规则。升级 Agent 前应先升级 HUB，使 HUB 返回包含规范节点编码的新认证挑战。

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

HUB 模式下，本机配置只负责隧道接口名、WireGuard UDP 监听端口、私钥和 HUB 连接参数；隧道地址、Peer、关联站点网段、NAT 和 ACL 由 HUB 从 Management 当前业务数据编译。Gateway 必须先分配 IPAM 隧道地址，且关联站点网络应有本机物理或静态路由。没有匹配 Gateway 的活动 `allow` 规则时，转发默认拒绝。首个有效快照应用前转发保持阻断。业务数据修改后，HUB 重新编译配置并通知 Gateway 对账。

以前台方式启动：

```sh
sudo /usr/local/bin/gateway-agent run \
  --config /etc/velotrust/gateway/gateway.json \
  --health-listen 127.0.0.1:8765 \
  --log-level debug
```

Gateway Agent 会在 `listen_port` 上启动 WireGuard UDP 监听，并应用 Peer、路由和防火墙规则。需在 Gateway 主机防火墙及云安全组放行 UDP `51820`。运行用户还需要创建隧道、配置路由、nftables 和 IP 转发的权限。空 `rules` 表示默认拒绝转发。

`tunnel.mtu` 以字节为单位，省略时 Gateway 使用 1420；客户端省略时使用 1380。HUB 快照省略 MTU 时 Gateway 保留本机配置值。关联网络 `routes` 表示 Gateway 后方的私网目的网段；Gateway 使用主机已有路由选择出口，不会为站点网段额外安装重复路由。HUB 自动生成从 Overlay 全局网段到每个活动关联站点的 Masquerade，以便后端响应返回 Gateway；当前 Overlay 仅支持 IPv4，关联 IPv6 站点会被拒绝，直到 IPv6 Overlay SNAT 可用。影子网段冲突时再额外配置 DNAT。Peer 的 `allowed_ips` 则描述该 Peer 对应的隧道地址和远端路由网段。

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
- Client `allowed_ips` 生成 Windows 隧道路由；Gateway Peer `allowed_ips` 生成 Linux WireGuard 路由。Gateway 顶层 `routes` 是其后方的站点网段；Agent 验证主机已有路由能够选择有效物理出口，不会为站点网段添加重复主机路由。
- HUB 管理的 Gateway 会自动对 Overlay 客户端访问关联站点的流量做 Masquerade，后端不需要额外配置 Overlay 回程路由。经其他 Gateway 可达的网段应放在对应 Peer 的 `allowed_ips` 中，由 Agent 安装到 WireGuard 接口。
- `rules` 必须显式允许需要的流量；只配置 WireGuard Peer 不会自动授权访问资源。

Client 与 Gateway 的字段定义、Endpoint 角色、AllowedIPs、路由与 MTU 约定见[网络配置契约](VeloTrust-网络配置契约.md)。

#### 静态 Gateway 配置 SNAT

HUB 管理模式会从 Overlay 全局网段和 Gateway 关联站点自动生成 Masquerade，不要再为该站点手工添加重复 SNAT。以下配置仅适用于不使用 HUB 下发关联站点配置的静态 Gateway。它要求 LAN 主机没有返回客户端隧道网段的路由，并让 Gateway 使用其 LAN 地址作为客户端流量的来源地址。先确认连接 LAN 的接口和地址：

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

规则应显示隧道入口、指定出口、源/目标前缀和 `snat to` 地址。可在出口接口抓包确认 LAN 主机看到的来源地址是 `to_address`；回包应经 Gateway 的连接跟踪还原后进入 WireGuard 隧道。

## Management 控制台

### 配置访问策略与 Gateway ACL

先在“企业资源管理中心”登记受保护服务：选择网络站点、填写 IPv4 地址或 CIDR，并设置协议和单端口、端口范围或任意端口。目标地址必须完整落在该站点的真实网段内；如果站点启用了影子网段 DNAT，也可以填写完整落在该影子网段内的地址。任意端口仍只匹配所选的 TCP 或 UDP 协议。随后在“零信任安全策略 → 访问策略与 ACL”选择安全用户组、企业资源和允许/拒绝动作。策略页面不填写客户端或资源网段。

HUB 根据安全用户组成员、活动设备和 Overlay 租约计算 ACL 来源；根据企业资源的网络站点找到 Gateway，并从资源记录读取目标地址、协议和端口。若目标地址位于影子网段，HUB 会按站点 NAT 映射把它转换成 DNAT 后的真实目标，供 Gateway 转发 ACL 匹配。Gateway Peer、Client 路由及 ACL 都由活动策略关系驱动；Gateway 默认拒绝没有匹配 allow 规则的流量。

例如站点真实网段为 `172.16.0.0/16`，影子网段为 `172.31.0.0/16`，SSH 主机真实地址为 `172.16.2.5`：

- 企业资源：地址 `172.31.2.5`，协议 TCP，端口 22，选择该站点
- 访问策略：选择允许访问的安全用户组和此 SSH 资源，添加“允许”动作并启用策略

保存或启用策略后，HUB 会通知相关 Client 和 Gateway 更新配置。可在节点详情检查下发与实际配置摘要是否一致，再从组内 Client 验证：

```powershell
Test-NetConnection 172.31.2.5 -Port 22
```

Client 使用 `172.31.0.0/16` 影子路由；Gateway 先将目标 DNAT 到 `172.16.2.5`，再由编译后的 ACL 按组内设备 Overlay 地址、真实目标和 TCP/22 匹配。SNAT 只负责回程地址转换，不会代替策略授权。

部署此版本前先执行 Management 数据库迁移。原来直接填写源/目标网段的活动策略无法安全转换为资源授权策略，迁移会将其改为草稿；请在企业资源管理中心登记服务，再重新关联安全用户组和资源后启用。

## Windows Client CLI

Windows Client 有两种连接方式：推荐的 **HUB 管理模式**由员工账号密码登录，HUB 完成设备登记、身份认证和配置下发；静态模式由管理员把 Gateway Peer 写入本机配置，适合现有手工联调。

### 运行前准备

将已部署的 `client-cli-windows-amd64.exe` 和 x64 `wintun.dll` 放在同一目录。生成密钥时使用普通 PowerShell；连接和断开隧道时使用**以管理员身份运行**、且与生成密钥相同 Windows 用户的 PowerShell。

```powershell
$Exe = "C:\VeloTrust\Client\client-cli-windows-amd64.exe"
& $Exe version
```

Client CLI 以当前进程前台运行，当前不是 Windows 后台服务。

### HUB 管理模式

#### 创建员工登录账号

管理员先在 Management 控制台「零信任安全策略 → 员工用户管理」创建员工账号并设置初始密码。员工账号不能登录 Management 控制台，只用于 Client Agent 登录。员工可在该页面由管理员重置 Client 登录密码；重置后需更新员工设备上正在使用的密码。

#### 创建 Client 配置

在 Windows 上生成设备私钥。这个密钥既用于 HUB 的 X25519 身份认证，也作为 WireGuard 隧道私钥；私钥仅保存在本机。

```powershell
$ClientDir = Join-Path $env:LOCALAPPDATA "VeloTrust\Client"
& $Exe keygen --dir $ClientDir
```

创建 `$ClientDir\client.json`，配置 HUB 地址。Client 首次连接时会提示输入员工账号和密码；密码不会写入此配置文件。

```json
{
  "schema_version": 1,
  "instance_id": "example-enterprise",
  "identity": {
    "private_key_file": "client.key"
  },
  "hub": {
    "endpoint": "10.1.3.59:8081",
    "server_name": "hub.example.com",
    "ca_file": "hub-ca.pem"
  }
}
```

Client 和 HUB 默认使用 TLS。仅在隔离的开发网络中，可同时在两端显式关闭 TLS：Client 在 `hub` 中设置 `"tls_enabled": false`，HUB 设置 `"tls": {"enabled": false}`。此时员工密码会通过明文 gRPC 传输，不得用于生产网络或不可信网络。HUB 连接参数和设备私钥路径由本地管理员设置；隧道地址、Gateway 公钥、Endpoint 和获准路由由 HUB 下发，不要在 HUB 模式本机配置中另写静态 `tunnel`。

校验并连接：

```powershell
$ClientConfig = Join-Path $ClientDir "client.json"
& $Exe config validate --file $ClientConfig --json
& $Exe connect --config $ClientConfig --log-level debug
```

连接时需使用管理员 PowerShell。Client 提示输入员工用户名和密码（密码不回显）；也可通过 `--username` 预先指定用户名。首次成功登录时，HUB 校验员工账号，并自动登记本机公钥、设备和节点；每次连接都校验员工密码，再通过 X25519 挑战应答证明设备私钥持有权。账号密码只通过 TLS 发送给 HUB，控制流不请求 Management 地址或 Management API。重连时仍需输入员工账号密码；如果密码被管理员重置，需使用新密码登录。

首次登录后，HUB 会根据设备所有者的活动用户组，查找 IPAM 动态 `User-Device` 地址池的员工组关联并创建地址租约；重连时复用有效租约。此步骤只分配 IP，不选择 Gateway，也不创建隧道 Peer。Client 可连接的 Gateway 应由访问策略决定；策略配置下发尚未完成前，HUB 不会下发隧道配置。若没有获得地址，请检查员工是否属于活动用户组、该组是否关联了启用的动态终端地址池，以及池中是否还有可用地址。收到有效 Gateway 配置后，Client 会创建 `VeloTrust` 接口并显示各 Gateway 握手状态。随后可按策略测试获准资源：

管理员先在「零信任安全策略 → 安全用户组」创建用户组并加入员工，再在「IPAM → Overlay 地址池」创建动态 `User-Device` 地址池并选择该用户组。Overlay 地址池分配隧道 IP；访问策略关联用户组和企业资源后，HUB 会为该组相关设备生成 Gateway Peer、路由及 ACL。现有“Gateway 准入”绑定仍可作为额外的组级准入配置。

```powershell
Test-NetConnection 10.20.0.10 -Port 443
```

管理员禁用 Client 节点、设备或所属用户后，在线 Client 收到 HUB 撤销通知会清理本机隧道；离线 Client 下次连接时认证会被拒绝。按 Ctrl+C 正常停止；异常退出后可手动清理：

```powershell
& $Exe disconnect --config $ClientConfig
```

### 静态配置模式

静态模式不会连接 HUB、不会使用注册 Token，也不会从控制台同步隧道配置。管理员需在 Gateway Peer 中手动登记客户端公钥。

#### 生成客户端密钥

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

HUB 管理模式已有员工密码登录、Client 设备首次登记、X25519 身份认证、配置同步、运行状态上报和在线撤销清理。HUB 会话租约仍是单实例内存状态。Windows Client 仍以前台进程运行，macOS/Linux DNS 接管、Windows 网络切换验证及休眠恢复尚未实现。DNS 接管通过 Windows DNS Client/NRPT 工作；自行发送 DNS、DoH 和 DoT 的应用不在当前范围。Management 当前访问策略已编译为 Gateway ACL；没有匹配允许规则的转发仍默认拒绝。
