# signal compatibility placeholder

南向控制通道、节点会话、配置分发和运行状态上报统一归属 [`../hub`](../hub/README.md)。当前 `signal/` 与 `cmd/signal/` 仅保留旧目录占位，不再作为独立服务实现入口，避免与 HUB 重复维护业务逻辑。

跨进程契约统一放在 `../shared/proto/`，HUB 生成的 Go 代码放在 `../shared/gen/go/`。
