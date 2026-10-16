# client

跨平台终端 Agent，负责设备身份与合规状态、南向控制连接、WireGuard 隧道、Split DNS 和终端进程特征采集。当前已提供 Windows amd64 CLI，并支持通过一个 WireGuard 接口同时连接多个 Gateway Peer；旧的单 `peer` 配置仍兼容。

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
client-cli.exe connect --config "$env:LOCALAPPDATA\VeloTrust\Client\client.json"
client-cli.exe disconnect --config "$env:LOCALAPPDATA\VeloTrust\Client\client.json"
```

Windows 构建：`make build-client-cli-windows`，生成 `bin/client-cli-windows-amd64.exe`。`connect` 需要管理员权限和同目录的 `wintun.dll`，会在前台运行，按 Ctrl+C 断开并清理本次地址与路由。`tunnel.peers` 中每个 Gateway 有独立 endpoint、公钥和 `allowed_ips`；精确相同的路由不能分配给多个 Gateway，网段冲突应使用各自不同的 NAT 映射网段。目前支持同时连接不同资源网段，尚不提供同一网段的主备故障切换。可用 `connect --log-level debug` 查看每个 Gateway 的 Endpoint、公钥指纹、Peer 状态和握手统计；日志不包含私钥。`disconnect` 可在进程被强制结束后清理残留路由与地址。当前连接依赖管理员预置的静态 Peer 配置；用户登录、设备注册、控制面授权配置同步和后台 Windows 服务尚未实现。服务状态不能代替隧道握手或资源可达状态。

详细测试步骤见[Windows Client CLI 测试手册](../docs/mannual.md)。
