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

Management 使用模板中的 `app/config/handler/repository/service` 等目录组织代码，并通过 `pkg/ioc` 注入依赖；`shared/repository/database` 封装 XORM 连接池、健康检查和事务生命周期。IAM 是首个接入该框架的业务模块，负责本地账号、Argon2id 密码、Passkey、HS256 JWT Bearer 令牌、可撤销服务端会话、登录限流和静态 RBAC。JWT 包含签发者、受众、用户 ID、会话 ID 和绝对过期时间；服务端仍核验对应会话，以便立即生效注销、账号停用和空闲超时。

IAM 的 JRPC 方法使用 `IAM_Method` 命名。登录、Passkey 登录是公开方法；其他方法先解析数据库中的活动会话，再检查代码中的权限映射。未知或未声明的 IAM 方法默认拒绝。角色授权在事务内检查操作者有效权限，并禁止自我授予角色。

当前 Web 管理台已对接的 IAM JRPC 方法包括：

- 会话：`Login`、`Logout`、`GetCurrentUser`、`ChangePassword`
- 多因素认证：`SetupTOTP`、`ConfirmTOTP`、`DisableTOTP`
- Passkey：`BeginPasskeyLogin`、`FinishPasskeyLogin`、`BeginPasskeyRegistration`、`FinishPasskeyRegistration`、`RevokePasskey`
- 用户：`ListUsers`、`CreateUser`、`SetUserStatus`、`ResetUserPassword`、`ListUserRoles`、`GrantRole`、`RevokeRole`
- 角色和权限：`ListRoles`、`CreateRole`、`ListPermissionCatalog`、`ListRolePermissions`、`GrantPermission`、`RevokePermission`
- 角色菜单：`ListRoleMenus`、`SetRoleMenus`、`ListCurrentUserMenus`。菜单变更整组事务提交并写入审计；系统角色菜单只读，普通角色可在控制台树形选择。
- 菜单：`ListMenus`、`CreateMenu`、`UpdateMenu`、`DeleteMenu`。菜单写入使用 `iam.role.write`，更新带版本号进行并发保护；删除会拒绝仍有子菜单或角色绑定的节点。菜单关联只控制导航显示，不能代替服务端权限码校验。
- 网关注册凭证：`ENROLLMENT_ListCredentials`、`ListSelectableSiteNetworks`、`CreateGateway`、`ExtendCredential`、`RevokeCredential`。凭证明文只在创建网关时返回一次，数据库仅存 SHA-256 摘要；读写分别要求 `gateway.registration.read` 和 `gateway.registration.write`，所有管理操作都写入 IAM 审计日志。凭证必须随新网关身份一并创建，不提供为已有网关重新签发的入口。
- 网络站点：`SITE_ListSites`、`ListGateways`、`SaveSite`、`DeleteSite` 管理 `network_type='site'` 的网络记录；读写需要 `ztna.site.read` / `ztna.site.write`。站点 ID 就是 network ID；网关注册页仍通过 `ENROLLMENT_ListSelectableSiteNetworks` 获取启用的站点网段并提交该 ID。

Gateway inventory 和凭证表通过 `006_gateway_enrollment.sql` 创建，权限通过 `007_gateway_registration_permissions.sql` 加入，并授予活动的 `platform_admin` 角色。网关注册凭证页面已接入这些 JRPC 接口。Gateway Agent 端的凭证消费、节点身份认证、注册审批和注册后配置下发尚未实现；创建网关并签发凭证目前只完成凭证管理，不代表 Gateway 已完成入网。

网络站点直接保存为 `t_ztna_network` 中 `network_type='site'` 的记录；站点 ID、名称、CIDR、可选网关、状态和逻辑删除标记都由同一行维护。迁移 013 为网络目录增加站点字段和站点权限，不创建单独的站点表。站点创建后名称和 CIDR 固定，编辑只修改 Gateway 关联和状态；停用或删除站点会同时停用关联 NAT，删除还会清空 Gateway 关联并逻辑删除站点。站点 NAT 通过 `SITE_SaveNATMapping` 和 `ztna.site.write` 管理，持久化于 `t_ztna_nat_mapping`，启用映射由 HUB 编译并同步至 Gateway Agent 数据面。

`t_ztna_overlay_network` 保存单例 Overlay 全局网段，管理员须先在 IPAM 页面初始化其 `10.0.0.0/8` 私有 Class A 地址空间中的 CIDR，之后才能创建地址池；`t_ztna_address_pool` 和 `t_ztna_address_lease` 管理该范围内的 IPv4 Overlay 地址池、分配和释放历史。

IPAM JRPC 管理 Overlay 范围、地址池与租约；网络站点及其 NAT 由 Sites 服务管理。全局 Overlay CIDR 使用版本号防止并发覆盖，地址池与站点前缀在写事务中校验地址范围约束。schema 说明和可审阅 SQL 副本位于 `docs/database/postgresql/schema/ipam/`。

Migration `011_ztna_business_core.sql` 按最终数据模型创建 21 张 ZTNA 业务表，涵盖设备可信状态、控制节点、受保护资源、当前访问策略、授权记录、隧道 Peer、配置分发任务、运行状态和安全事件。IAM、企业网络目录、Gateway 注册和 IPAM 表已由前序 migration 创建，本次未重复定义。可审阅 SQL 副本位于 `docs/database/postgresql/schema/ztna/`；其中建议或按需启用的表目前只完成数据结构，相关业务服务仍需后续实现。

Migration `013_physical_sites.sql` 为 `t_ztna_network` 增加网络站点所需字段、逻辑删除标记和站点权限；后续 migration 014 将历史菜单和权限名称统一为“网络站点”。Migration 015 将网关身份和网关网络关系并入 `t_ztna_node` / `t_ztna_node_network`，迁移旧引用后删除 `t_gateway_node` 与 `t_gateway_node_network`，并将 IPAM 和注册凭证中的数值审计用户 ID 转为 IAM 用户名。已有 `network_type='site'` 记录直接作为站点记录，无需复制数据。

本地开发时，Web 的 Vite 服务默认将 `/jrpc` 代理到 `http://127.0.0.1:8080`。浏览器通过 `http://localhost:3000` 使用 Passkey 时，`[auth.webauthn]` 必须配置 `rp_id = "localhost"` 和 `origins = ["http://localhost:3000"]`；这些值需要与实际 Origin 完全匹配。生产环境改为控制台 HTTPS 主机名，并同步配置对应 RP ID 和 Origin。

IAM 模块的数据模型、会话和访问控制设计见 [IAM 设计文档](../docs/VeloTrust_IAM身份与访问管理模块设计.md)。版本化 PostgreSQL 迁移位于 `management/migrations/postgresql/`；初始 schema 快照仍保存在 `docs/database/postgresql/schema/iam/` 供审阅。

IAM、IPAM、Gateway 注册、ZTNA 和定时任务的持久化模型统一嵌入 `database.BaseModel`；需要逻辑删除的模型嵌入 `database.DeletableModel`。Repository 的数据变更必须通过 `db.Save` 或事务内的 `tx.Save` 完成，读操作仍使用 Repository 查询；写事务使用 `WithSaveTransaction` 将校验、锁定、模型保存和审计写入放在同一事务中。更新会检查 `version_no`，冲突时返回错误。迁移 `012_model_save_metadata.sql` 为旧表补齐 BaseModel 所需的元数据列；`015_unify_gateway_nodes_and_audit_actors.sql` 修正遗留数值审计字段并合并重复网关表。

控制台菜单 seed migration 位于 `management/migrations/postgresql/004_iam_menu_seed.sql`；前置 migration `003_iam_menu_button_type.sql` 支持原型中的按钮级菜单项。`005_iam_role_menu_seed.sql` 为已有活动角色初始化当前菜单关联，bootstrap 则在事务中为平台管理员关联全部活动菜单。对应的独立 SQL 副本保存在 [seed 文档目录](../docs/database/postgresql/seed/)。

## 首次运行

1. 设置数据库连接、随机 JWT 签名密钥、限流 HMAC 密钥和 TOTP 加密密钥：

   ```sh
   export DATABASE_DSN='postgres://velotrust:secret@127.0.0.1:5432/velotrust?sslmode=disable'
   export VELOTRUST_JWT_SIGNING_KEY="$(openssl rand -hex 32)"
   export VELOTRUST_THROTTLE_HMAC_KEY="$(openssl rand -hex 32)"
   export VELOTRUST_TOTP_ENCRYPTION_KEY="$(openssl rand -hex 32)"
   ```

   三个密钥都需通过受保护的密钥管理方式保存。JWT 签名密钥在重启、升级和所有 Management 副本之间必须保持一致；丢失或更换会使现存 JWT 失效。TOTP 加密密钥丢失或更换后，已存储的 TOTP 密钥将无法解密。

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

Migration `016_network_gateway_ownership.sql` 将网络网关归属统一至 `t_ztna_network.gateway_node_id`，迁移唯一且一致的绑定后删除 `t_ztna_node_network`。多网关归属、两处绑定冲突、非网关关系或 CIDR 快照差异会阻止迁移并给出记录 ID；核对修复后重试。请先执行 `management migrate --config ...`，再启动新版 Management 与 Web；站点 JRPC 字段同步改为 `gateway_node_id`。
