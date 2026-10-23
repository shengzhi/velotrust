# client

跨平台终端 Agent，负责设备身份与合规状态、南向控制连接、WireGuard 隧道、Split DNS 和终端进程特征采集。当前已提供 Windows amd64 CLI，并支持通过一个 WireGuard 接口连接多个 Gateway Peer。连接方式分为 HUB 管理模式和静态配置兼容模式。

- `cmd/client-cli`：客户端命令行入口，提供版本、Agent 服务状态、配置校验、诊断、密钥生成和前台隧道连接。
- `internal`：生命周期、配置同步、Peer、隧道、防火墙、DNS、Posture 等平台无关逻辑。
- `internal/tunnel/wireguard`：Windows WireGuard-Go/Wintun 客户端隧道。
- `iface`：隧道接口适配。
- `system/{windows,macos,linux}`：操作系统适配。
- `packaging`：各操作系统安装包与发布资料。

策略决策留在控制面；`internal/firewall` 只负责应用下发规则。设备私钥由终端侧保护，控制连接与普通业务流量保持分离。

当前 CLI 命令：

```powershell
client-cli.exe version
client-cli.exe status --json
client-cli.exe config validate --file .\client\configs\client.example.json
client-cli.exe config validate --file "$env:ProgramData\VeloTrust\Client\client.json"
client-cli.exe diagnose --json
client-cli.exe keygen --dir "$env:LOCALAPPDATA\VeloTrust\Client"
client-cli.exe connect --config "$env:LOCALAPPDATA\VeloTrust\Client\client.json" --username employee01
client-cli.exe disconnect --config "$env:LOCALAPPDATA\VeloTrust\Client\client.json"
```

Windows 构建：`make build-client-cli-windows`，生成 `bin/client-cli-windows-amd64.exe`。`connect` 需要管理员权限和同目录的 `wintun.dll`，会在前台运行。静态模式使用本机配置中的 `tunnel`；HUB 管理模式使用 `hub.endpoint` 与 `identity.private_key_file`，提示员工输入账号密码，并通过 HUB 完成身份认证、首次设备登记和隧道快照同步，不调用 Management API。HUB 模式下本地配置不需要 `tunnel` 字段，默认启用 TLS。仅在隔离的开发网络中，Client 和 HUB 可同时显式关闭 TLS；该模式会明文传输员工密码，不能用于生产网络或不可信网络。

员工账号在 Management 的「员工用户管理」页面单独创建和重置密码。员工在 Client 首次登录时提交账号密码和本机 X25519/WireGuard 公钥；HUB 校验员工账号后，在事务中登记节点、设备和设备公钥凭证。后续连接继续使用账号密码登录，并以 X25519 挑战证明设备私钥持有权。禁用 Client 节点、设备或所属员工会拒绝后续认证，并通知在线 Client 清理隧道。当前 HUB 配置编译要求 Client 与 Gateway 已加入同一个活动逻辑隧道；Management 尚未提供隧道 Peer 关联界面，需由运维侧配置现有隧道关系后才能下发完整数据面配置。

Windows Client 可通过 `tunnel.dns.resolver_groups` 启用 Split DNS：系统 DNS 查询经 NRPT 导入本地代理，企业域名按最长后缀匹配转发到企业 DNS，其他域名使用当前系统 DNS 上游。企业 DNS 地址必须被某个 Peer 的 `allowed_ips` 覆盖。Debug 日志显示所选上游类别，不输出私钥。正常断开会移除本实例的 DNS 接管规则；若进程被强制结束，使用 `disconnect` 清理残留 NRPT 规则、路由和地址。DoH/DoT 与应用自建 DNS 不由当前版本拦截。

Client 与 Gateway 的地址、公钥、Endpoint、AllowedIPs、路由和 MTU 约定见[网络配置契约](../docs/VeloTrust-网络配置契约.md)。

当前 CLI 仍以前台进程运行，Windows 后台服务和多实例 HUB 会话共享尚未实现。Client 控制流只连接 HUB；员工账号由管理员在 Management 创建，员工密码仅用于 HUB 上的 Client 登录，不授予 Management 控制台登录权限。服务状态不能代替隧道握手或资源可达状态。

详细测试步骤见[Windows Client CLI 测试手册](../docs/mannual.md)。
