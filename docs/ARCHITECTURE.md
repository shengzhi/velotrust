# VeloTrust Monorepo 架构

VeloTrust 使用单一 Go monorepo。控制面初期采用模块化单体；Agent、信令、中继及网络代理按清晰的模块边界组织，需要时再独立部署。跨进程契约集中在 `shared/`，管理服务的数据库实体不直接复用为 Agent 协议类型。

## 目录结构

```text
velotrust/
├── cmd/
│   ├── server/                 # 管理控制面入口
│   ├── agent/                  # 终端 Agent 入口
│   ├── cli/                    # 管理命令行
│   ├── signal/                 # 可独立部署的信令入口
│   ├── gateway-agent/          # 企业网络网关 Agent 入口
│   └── l7-proxy/               # 七层审计代理入口
├── management/
│   └── internal/
│       ├── identity/ device/ group/ resource/ policy/
│       ├── network/ tunnel/ enrollment/ audit/ store/
│       └── transport/ worker/
├── client/
│   ├── cmd/                     # 客户端专属辅助入口
│   ├── internal/{engine,sync,peer,tunnel,firewall,dns,posture}/
│   ├── iface/                   # 隧道接口适配
│   └── system/{windows,macos,linux}/
├── signal/                      # 南向 gRPC 信令服务
├── relay/                       # 按需部署的中继服务
├── gateway/                     # WireGuard 网关与网络执行面
├── l7proxy/                     # 七层检查与审计
├── shared/{proto,api,auth,model}/
├── pkg/{wgctrl,firewall,platform}/
├── web/                          # 管理控制台与自助门户
├── migrations/                  # 数据库迁移
├── deployments/                 # 自托管部署配置
├── scripts/                     # 开发与代码生成脚本
├── docs/                        # 技术方案与架构文档
├── go.mod
└── Makefile
```

`gateway/` 和 `l7proxy/` 保留原技术方案中的网关与七层代理职责；它们是对基础目录树的补充，不与终端 `client/` 混用。

## 领域与执行边界

- `management/internal/policy` 负责策略匹配和授权决策，并以数据库当前记录作为策略权威状态；`client/internal/firewall` 与 `gateway/internal/firewall` 将已下发策略转换为本机或网关规则。
- `management/internal/tunnel` 负责期望隧道配置、Peer 管理和配置生成；`client/internal/tunnel` 与 `gateway/internal/tunnel` 负责在数据面应用配置。
- `pkg/wgctrl` 封装 WireGuard 控制能力。平台接口与系统调用分别经 `pkg/platform`、`client/system` 和 `gateway/internal/platform/linux` 适配。
- `shared/proto` 仅放跨进程通信契约；认证协议、API 类型和通用模型分别归 `shared/auth`、`shared/api`、`shared/model`。持久化实体留在 `management` 内部。
- `management` 初期作为模块化单体部署。`signal`、`relay` 和数据面组件可按需独立部署，不为组件拆分而预先引入复杂分布式依赖。
- 组织、用户组和设备组等业务模型按需保留；当前不引入复杂的 `tenant_id` 隔离体系。

## WireGuard 代码位置

WireGuard 的配置生成和运行时应用明确分层：控制面在 `management/internal/tunnel` 维护期望状态；终端与网关各自在 `client/internal/tunnel`、`gateway/internal/tunnel` 应用状态；`pkg/wgctrl` 提供底层控制封装。具体系统调用留在平台适配层，策略业务逻辑不进入 WireGuard 或防火墙适配器。
