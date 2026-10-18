# Go 项目模板集成记录

VeloTrust 将 `/data/project/code-templates/go-template` 中可复用的通用工程框架接入 monorepo，并按当前 JRPC、IAM schema 和运行时边界适配。模板旧版 IAM 业务对象没有直接替换 VeloTrust 的用户、会话、Passkey 或 RBAC 模型。

## 目录迁移

| 模板职责 | VeloTrust 目标目录 | 集成方式 |
| --- | --- | --- |
| 应用组合、配置、服务启动 | `management/internal/app/`、`management/internal/config/`、`management/app/` | 保留 Management 现有 runtime/JRPC 入口，由 `management/app` 暴露组合根 |
| Handler、Middleware、Service、Repository | `management/internal/{handler,middleware,service,repository}/` | 复用分层目录，IAM 使用独立模型和 Repository 接口 |
| 配置模型与调度 | `management/internal/config/`、`management/internal/schedule/` | 采用模板结构，配置补充环境变量密钥解析 |
| 通用模型、i18n、日志和 OSS | `management/internal/{model,i18n,lib}/` | 保留与当前 Management 仍有关联的通用组件 |
| 常量 | `shared/constants/` | 放置跨模块常量 |
| 数据库访问 | `shared/repository/database/` | 保留 XORM 包装并补充 Ping、受控连接池和显式事务生命周期 |
| IOC | `pkg/ioc/` | 复用模板 IOC 目录和注入形式，改为错误返回并检测循环依赖 |

模板的 `pkg/jrpc` 没有覆盖 VeloTrust 当前 JRPC 运行时，因此继续使用 `pkg/jrpc` 中已有实现，不复制第二套 RPC 协议栈。模板遗留的 JWT/OTP IAM 数据实体没有接入当前 IAM schema。

## 数据库与事务

`shared/repository/database.Database` 是共享连接池和事务组件；IAM SQL 只在 `management/internal/repository/iam` 内，业务服务仅依赖 `repository.Store`。IAM 当前明确要求 PostgreSQL，因为登录限流、挑战消费和 RBAC 使用 PostgreSQL UPSERT、JSONB 与事务语义。其他模板数据库驱动仍留在通用 wrapper 中，但 IAM 不会尝试在 MySQL/SQLite 上降级运行。

配置中的数据库 DSN、限流 HMAC 密钥及 Redis 密码支持 `${ENV_VAR}` 展开。Passkey 临时状态可以使用本机内存存储（单实例）或 Redis 的原子 GETDEL（多实例）。

Management 默认忽略来自未信任网络对端的 `X-Forwarded-For` 和 `X-Real-IP`。反向代理部署应在 `http.trusted_proxies` 中仅列出代理 IP/CIDR；这也避免通过伪造转发头绕过 IAM 来源地址限流。

## 生命周期

Management 进程内通过 IOC 以 singleton 注册 Config、数据库、IAM Store 和 Service。当前 IOC 组合根只允许每个进程初始化一次；多个 Management 实例应分别运行并共享 PostgreSQL/Redis，不应在同一进程重复创建多个 Management Runtime。

数据库或身份状态无法验证、权限映射缺失、Passkey challenge 失效或被消费时，IAM 采取拒绝访问。运行服务前应先通过版本化迁移流程部署 schema；当前 `docs/database/postgresql/schema/iam/001_iam.sql` 仍是空数据库 schema 快照，尚未执行。
