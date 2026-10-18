# Management

Management 是管理控制平面，负责管理控制台、用户与角色、网络与资源、策略配置、设备管理和审计。它维护期望状态，不承载普通用户业务流量。

## JRPC 接口

Management 使用 `pkg/jrpc` 提供的 HTTP JRPC 协议；HUB 的 Agent 长连接仍使用 gRPC，二者职责分开。

- 地址：`POST /jrpc`
- Content-Type：`application/json`
- 方法命名：`Service_Method`，例如 `System_Version`
- 参数放在 `params` 对象中；`id` 原样返回。
- 默认需要身份认证；显式标记为 public 的方法允许匿名调用（当前包括 `System_Version` 和 IAM 的密码/Passkey 登录入口）。
- 请求体限制默认 1 MiB。

```json
{"jsonrpc":"2.0","id":"req-001","method":"System_Version","params":{}}
```

当前模板 JRPC 实现的实际成功响应格式为：

```json
{"result":{"service":"management","version":"dev"},"error":null,"code":200,"id":"req-001"}
```

模板的前端文档示例声明了 `jsonrpc` 响应字段，但模板服务端当前未输出该字段，并额外输出 `code`。此处先保持与模板服务端代码兼容；不要将它描述为严格的 JSON-RPC 2.0 响应。后续如需改变线协议，应单独做兼容性版本设计。

## 目录职责

- `runtime/`：独立服务生命周期；未来可由组合启动入口与 HUB 一起运行。
- `internal/transport/jrpc/`：HTTP 监听、健康端点和 JRPC 服务注册。
- `internal/{identity,device,policy,network,...}/`：Management 业务模块，各模块通过 Repository 访问持久化数据。
- `configs/`：非敏感配置模板。
- `../pkg/jrpc/`：参考 Go 模板中的 `pkg/jrpc` 并适配到 monorepo 的协议运行时。

Management 使用模板中的 `app/config/handler/repository/service` 等目录组织代码，并通过 `pkg/ioc` 注入依赖；`shared/repository/database` 封装 XORM 连接池、健康检查和事务生命周期。IAM 是首个接入该框架的业务模块，负责本地账号、Argon2id 密码、Passkey、可撤销 Bearer 会话、登录限流和静态 RBAC。

IAM 的 JRPC 方法使用 `IAM_Method` 命名。登录、Passkey 登录是公开方法；其他方法先解析数据库中的活动会话，再检查代码中的权限映射。未知或未声明的 IAM 方法默认拒绝。角色授权在事务内检查操作者有效权限，并禁止自我授予角色。

当前 Web 管理台已对接的 IAM JRPC 方法包括：

- 会话：`Login`、`Logout`、`GetCurrentUser`、`ChangePassword`
- 多因素认证：`SetupTOTP`、`ConfirmTOTP`、`DisableTOTP`
- Passkey：`BeginPasskeyLogin`、`FinishPasskeyLogin`、`BeginPasskeyRegistration`、`FinishPasskeyRegistration`、`RevokePasskey`
- 用户：`ListUsers`、`CreateUser`、`SetUserStatus`、`ResetUserPassword`、`ListUserRoles`、`GrantRole`、`RevokeRole`
- 角色和权限：`ListRoles`、`CreateRole`、`ListPermissionCatalog`、`ListRolePermissions`、`GrantPermission`、`RevokePermission`
- 角色菜单：`ListRoleMenus`、`SetRoleMenus`、`ListCurrentUserMenus`。菜单变更整组事务提交并写入审计；系统角色菜单只读，普通角色可在控制台树形选择。
- 菜单：`ListMenus`、`CreateMenu`、`UpdateMenu`、`DeleteMenu`。菜单写入使用 `iam.role.write`，更新带版本号进行并发保护；删除会拒绝仍有子菜单或角色绑定的节点。菜单关联只控制导航显示，不能代替服务端权限码校验。
- 网关注册凭证：`ENROLLMENT_ListGateways`、`ListCredentials`、`CreateGateway`、`IssueCredential`、`ExtendCredential`、`RevokeCredential`。凭证明文只在签发响应中返回一次，数据库仅存 SHA-256 摘要；读写分别要求 `gateway.registration.read` 和 `gateway.registration.write`，所有管理操作都写入 IAM 审计日志。
- 物理站点网段：网关注册页通过 `ENROLLMENT_ListSelectableSiteNetworks` 读取 `t_ztna_network` 中启用的 `network_type='site'` 网段，并以 network ID 提交选择；后端会再次校验所选记录仍是启用的站点网段。

Gateway inventory 和凭证表通过 `006_gateway_enrollment.sql` 创建，权限通过 `007_gateway_registration_permissions.sql` 加入，并授予活动的 `platform_admin` 角色。网关注册凭证页面已接入这些 JRPC 接口。Gateway Agent 端的凭证消费、节点身份认证、注册审批和注册后配置下发尚未实现；签发凭证目前只完成凭证管理，不代表 Gateway 已完成入网。

`t_ztna_network` 保存 Gateway 可路由的网络前缀；网关签发只允许选择 `network_type='site'` 的启用记录，并在服务端再次校验、保存 CIDR 快照和关联。物理站点页面当前仍使用前端原型数据，尚未写入该目录；需通过站点持久化接口保存的站点网段才会出现在签发列表中。`t_ztna_overlay_network` 保存单例 Overlay 全局网段，管理员须先在 IPAM 页面初始化其 `10.0.0.0/8` 私有 Class A 地址空间中的 CIDR，之后才能创建地址池；`t_ztna_address_pool` 和 `t_ztna_address_lease` 管理该范围内的 IPv4 Overlay 地址池、分配和释放历史。NAT 映射保存真实网段与虚拟影子网段配置。

IPAM JRPC 包含 `Overview`、`SaveOverlayNetwork`、业务网段与 Overlay 地址池 CRUD/状态管理、`AllocateAddress`、`ReleaseLease`、`CheckPrefix`、`ListNATMappings` 和 `SaveNATMapping`。所有操作要求 `ipam.network.read` 或 `ipam.network.write`，变更写审计日志；全局 Overlay CIDR 采用版本号防止并发覆盖，地址池 CIDR、业务网段及 NAT 影子网段的重叠在写事务中重新校验。NAT 映射目前是配置台账，尚未同步至 Gateway Agent 数据面，页面会明确显示该状态。schema 说明和可审阅 SQL 副本位于 `docs/database/postgresql/schema/ipam/`。

Migration `011_ztna_business_core.sql` 按最终数据模型创建 21 张 ZTNA 业务表，涵盖设备可信状态、控制节点、受保护资源、访问策略及版本、授权记录、隧道 Peer、配置分发任务、运行状态和安全事件。IAM、企业网络目录、Gateway 注册和 IPAM 表已由前序 migration 创建，本次未重复定义。可审阅 SQL 副本位于 `docs/database/postgresql/schema/ztna/`；其中建议或按需启用的表目前只完成数据结构，相关业务服务仍需后续实现。

本地开发时，Web 的 Vite 服务默认将 `/jrpc` 代理到 `http://127.0.0.1:8080`。浏览器通过 `http://localhost:3000` 使用 Passkey 时，`[auth.webauthn]` 必须配置 `rp_id = "localhost"` 和 `origins = ["http://localhost:3000"]`；这些值需要与实际 Origin 完全匹配。生产环境改为控制台 HTTPS 主机名，并同步配置对应 RP ID 和 Origin。

IAM 模块的数据模型、会话和访问控制设计见 [IAM 设计文档](../docs/VeloTrust_IAM身份与访问管理模块设计.md)。版本化 PostgreSQL 迁移位于 `management/migrations/postgresql/`；初始 schema 快照仍保存在 `docs/database/postgresql/schema/iam/` 供审阅。

IAM、IPAM、Gateway 注册、ZTNA 和定时任务的持久化模型统一嵌入 `database.BaseModel`；需要逻辑删除的模型嵌入 `database.DeletableModel`。Repository 的数据变更必须通过 `db.Save` 或事务内的 `tx.Save` 完成，读操作仍使用 Repository 查询；写事务使用 `WithSaveTransaction` 将校验、锁定、模型保存和审计写入放在同一事务中。更新会检查 `version_no`，冲突时返回错误。迁移 `012_model_save_metadata.sql` 为旧表补齐 BaseModel 所需的元数据列。

控制台菜单 seed migration 位于 `management/migrations/postgresql/004_iam_menu_seed.sql`；前置 migration `003_iam_menu_button_type.sql` 支持原型中的按钮级菜单项。`005_iam_role_menu_seed.sql` 为已有活动角色初始化当前菜单关联，bootstrap 则在事务中为平台管理员关联全部活动菜单。对应的独立 SQL 副本保存在 [seed 文档目录](../docs/database/postgresql/seed/)。

## 首次运行

1. 设置数据库连接、随机限流 HMAC 密钥和 TOTP 加密密钥：

   ```sh
   export DATABASE_DSN='postgres://velotrust:secret@127.0.0.1:5432/velotrust?sslmode=disable'
   export VELOTRUST_THROTTLE_HMAC_KEY="$(openssl rand -hex 32)"
   export VELOTRUST_TOTP_ENCRYPTION_KEY="$(openssl rand -hex 32)"
   ```

   TOTP 加密密钥需在重启和升级期间保持不变；丢失或更换后，已存储的 TOTP 密钥将无法解密。

2. 使用 PostgreSQL 连接环境变量或权限为 `0600` 的 `.pgpass` 创建空数据库并执行版本化迁移：

   ```sh
   go run ./cmd/management migrate --config management/configs/management.example.toml
   ```

   若 IAM 表此前已手工创建，先检查其与 schema 快照一致，再添加 `--baseline-existing-iam` 记录初始版本；该选项不会重建现有表。

3. 创建首个管理员。初始密码交互读取后通过环境变量传入，不会写入配置文件或进程参数：

   ```sh
   read -s -r -p 'Initial administrator password (at least 8 characters): ' VELOTRUST_BOOTSTRAP_PASSWORD; printf '\n'; export VELOTRUST_BOOTSTRAP_PASSWORD
   ./bin/management bootstrap --config management/configs/management.example.toml --username admin
   unset VELOTRUST_BOOTSTRAP_PASSWORD
   ```

4. 配置 Passkey 的 `rp_id` 和 `origins` 使其与实际控制台域名完全匹配，然后启动服务：

```sh
make build-management
./bin/management --config management/configs/management.example.toml --listen 127.0.0.1:8080
```

未配置 Redis 时 Passkey ceremony 临时状态保存在本进程，仅适用于单实例运行；多实例 Management 必须配置共享 Redis。数据库或共享 ceremony store 不可用时，认证和 Passkey 验证失败关闭。

HTTP 监听默认绑定回环地址。生产环境由受控反向代理提供 TLS；不要通过明文 HTTP 传输认证令牌。管理 API 使用 `POST /jrpc`。

如果 Management 位于反向代理后，只有将代理的确切 IP/CIDR 配入 `http.trusted_proxies` 后，才会读取 `X-Forwarded-For`/`X-Real-IP`。未配置时服务只使用 TCP 对端地址，避免客户端伪造来源 IP 绕过登录限流。

登录请求示例：

```json
{"jsonrpc":"2.0","id":"1","method":"IAM_Login","params":{"username":"admin","password":"...","totp_code":"123456"}}
```

TOTP 配置通过 `IAM_SetupTOTP`（需重新输入密码）、`IAM_ConfirmTOTP` 和 `IAM_DisableTOTP` 管理。登录启用 TOTP 后必须带 `totp_code`。同一时间步验证码只能成功使用一次。管理员可用 `IAM_ResetUserPassword` 设置临时密码；用户必须调用 `IAM_ChangePassword` 后才能使用其他 IAM 接口。用户停用、最后一个平台管理员保护以及角色/权限撤销通过 `IAM_SetUserStatus`、`IAM_RevokeRole` 和 `IAM_RevokePermission` 完成；停用用户会在事务中撤销其所有会话。
