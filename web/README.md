# VeloTrust Web 管理台

Web 管理台的 IAM 登录与账号管理通过 Management 的 JRPC 接口访问，不在浏览器中保存模拟用户或自行判断授权。

## 本地运行

1. 按 [Management 首次运行说明](../management/README.md#首次运行) 完成数据库迁移、管理员初始化并启动 Management。
2. 确认 `management/configs/management.example.toml` 中 `[auth.webauthn]` 的 `rp_id` 与 `origins` 包含当前浏览器访问地址。使用 Vite 默认地址时设为：

   ```toml
   rp_id = "localhost"
   origins = ["http://localhost:3000"]
   ```

   Passkey 要求安全上下文；`localhost` 可用于本地开发。其他主机名请使用 HTTPS，并将确切的 Origin 加入 allowlist。
3. 在 `web/` 安装依赖并启动 Vite：

   ```sh
   npm install
   cp .env.example .env.local
   npm run dev
   ```

   `VITE_MANAGEMENT_JRPC_TARGET` 默认指向 `http://127.0.0.1:8080`，Vite 将 `/jrpc` 请求代理至 Management。若 Management 地址不同，修改 `.env.local` 后重启 Vite。
4. 打开 `http://localhost:3000`，使用 Management 中创建的 IAM 用户登录。首次管理员或临时密码登录后需先修改密码。

## 已接入的 IAM 功能

- 登录、退出、启动时验证 Bearer 会话、当前用户和权限读取。
- 用户查询、创建、启用/停用、临时密码重置、角色授予与撤销。
- 角色查询和创建；读取权限目录、为角色授予和撤销权限。
- 修改本人密码、配置/关闭 TOTP、Passkey 登录与注册/撤销。
- 页面访问按当前用户的 IAM 权限过滤；实际操作由 Management 服务端再次鉴权。

IAM Bearer token 仅存于当前标签页的 `sessionStorage`，当前 Web 实现不提供持久登录。401/会话失效后会清理会话并返回登录页。

## 尚未接入的页面

仪表盘、网络、策略、菜单、用户组、API 映射、日志等其他控制台模块仍可能显示原型数据。IAM 中后端尚未开放 JRPC 的功能会显示未实现状态，不会模拟写入；包括用户资料编辑、角色删除/停用、菜单资源目录管理、用户组管理和会话列表。

## 安全配置

- 不要把生产 Bearer token 放入 URL、日志或 localStorage。
- 生产环境通过 HTTPS 访问；Passkey 的 WebAuthn RP ID/Origin 必须与控制台主机名匹配。
- TOTP 加密密钥需在所有 Management 实例中一致且长期保存。
- Passkey 多实例部署需要共享 Redis ceremony store；单实例本地开发可用进程内存储。
- JRPC 的后端权限映射是最终授权边界，前端隐藏按钮只用于界面体验。
