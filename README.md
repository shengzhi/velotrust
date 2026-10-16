# VeloTrust

VeloTrust 是基于 WireGuard 的企业零信任访问平台。本仓库采用单一 Go monorepo，控制面以模块化单体起步，数据面与共享契约按目录边界组织。

- [项目架构](docs/ARCHITECTURE.md)
- [TODO LIST](TODO.md)
- [平台技术实现方案](docs/VeloTrust平台技术实现方案.md)
- [ZTNA 技术方案](docs/VeloTrust企业级零信任安全访问平台(ZTNA)技术方案.md)
- [Gateway Agent](gateway/README.md)
- [Client CLI](client/README.md)

平台模块职责、WireGuard 代码边界和跨进程协议归属见架构文档。
