## ⚖️ VeloTrust 企业级零信任安全访问平台（ZTNA）技术方案## 1. 引言与建设目标
VeloTrust 是基于 WireGuard 技术构建的、面向企业多云环境的零信任网络访问（ZTNA）与高性能组网系统。本方案遵循控制面（Control Plane）与数据面（Data Plane）完全解耦的架构设计，打破传统 VPN “一次认证、终身信任”与“大内网泛洪”的弊端。系统致力于为企业打造具备持续自适应准入、高合规深度审计、跨云重叠网段自愈以及百万级 Peer 动态无损加载能力的安全骨干网络。
------------------------------
## 2. 全局拓扑与系统物理架构
系统通过星型网格（Hub-and-Spoke）的变体实现强管控的南北向安全接入，整体部署架构如下图所示：
------------------------------
## 3. 逻辑子系统与业务模块划分
VeloTrust 完全体系统在逻辑架构上推荐划分为 5 个核心子系统，从而建立清晰的功能功能域边界：
## 🌐 子系统 1：虚拟网络与核心组网子系统 (Overlay Networking Subsystem)

* 模块 1：全局 Overlay 网络与自动化 IPAM 模块：管理全局虚拟网格，基于大网段（如 10.100.0.0/16）利用基数树（Radix Tree）在内存中执行原子性无冲突的网关和终端 IP 分配。
* 模块 2：智能网关 Agent 底层接口驱动模块：直接使用 Linux 原生 Netlink 机制与内核通信，支持网卡动态创建。配合 RPS（Receive Packet Steering）调优，将虚拟网卡的中断流均匀散列到所有 CPU 核心上，实现多队列网卡级别的并发加密性能。
* 模块 3：跨云重叠网段 NAT 映射模块：自动感知多云 VPC 冲突（如双方均使用 192.168.10.0/24），自动拨备“影子网段”（如 172.31.10.0/24），在内核态通过 NETMAP 扩展模块执行整段的位移映射，完成低损耗的双向 1:1 地址转换。

## 👥 子系统 2：身份集成与安全准入子系统 (Identity & Admission Subsystem)

* 模块 1：企业级第三方 IdP 联动模块：集成飞书、钉钉、Okta 等主流 OIDC 协议。解析 JWT 令牌中的属性标签，将其动态映射为系统内部的网络安全组成员。
* 模块 2：短期动态凭证与密钥轮转模块：对隧道会话执行 12 小时（TTL）强管制。到期后通过南向通道踢出节点，迫使客户端本地清除存储并在前端重新唤起 SSO 扫码认证。支持在不中断流量的情况下执行密钥级联轮转。
* 模块 3：持续自适应终端合规基线检查模块（Posture Check）：客户端守护进程每 5 分钟循环上报本地安全基线状态（防火墙、杀毒软件、活跃进程等），一旦不达标在 3 秒内下发 DisconnectOrder 触发动态降级或物理断连。

## 🛡️ 子系统 3：声明式微隔离策略子系统 (Zero-Trust ACL Policy Subsystem)

* 模块 1：基于身份的声明式 ACL 编排模块：提供“用户组 ➔ 目标子网/实体资产 ➔ 端口”的面向主体的 4 层过滤规则矩阵。
* 模块 2：端侧进程级应用流量限制模块：客户端 Agent 读取底层 Socket 特征，限制只有企业授信的应用（如 DataGrip.exe）发起的数据流才允许进入隧道。在端侧拦截非法横向渗透。
* 模块 3：边缘防火墙动态同步模块：控制面生成的策略秒级下发至多云网关，通过 nftables 或 eBPF 引擎执行热注入。

## 🗺️ 子系统 4：智能域名路由子系统 (Split DNS Subsystem)

* 模块 1：Split DNS 智能分流模块：在客户端注入轻量级解析代理，仅拦截匹配特定企业后缀（如 *.aliyun.internal）的 DNS 请求并走隧道定向投递，公网流量原路迭代，不暴露内网拓扑。
* 模块 2：内网视界隔离（DNS Views）管理模块：结合用户的权限组，动态提供不同维度的内网 DNS 解析记录。

## 📜 子系统 5：集中审计、可视化与高可用灾备子系统 (Audit & Resiliency Subsystem)

* 模块 1：七层应用级深度审计模块：网关集成了 7 层安全代理（如定制化 Envoy）。通过内核 TPROXY 协议将 4 层放行的特定端口流量透明重定向至代理中，执行高级 SQL 注入审计和 HTTP WAF 阻断，记录全量合规日志。
* 模块 2：全局可视化态势感知与一键熔断模块：提供全局拓扑大屏、UBA（用户行为分析）异常端口扫描告警，赋予管理员在发现凭证泄露时对高危设备执行一键拉黑熔断的权利。
* 模块 3：网关集群高可用（HA）与离线自决模块：采用双机热备（Active-Standby）模式结合云平台虚拟漂移 IP 机制。当网关 Agent 与控制面板失联时，启动死信定时器并激活本地只读缓存策略维持现有连接，若失联超时 30 分钟则自动进入自我熔断模式关闭网络。

------------------------------
## 4. 软件工程实体：应用模块拆解
上述 5 大逻辑子系统在代码中映射为以下模块，并统一维护在单一 `velotrust/` monorepo 中。模块边界与部署边界分开管理，初期不要求每个模块都独立部署。
## 4.1 控制面应用 (Control Plane)

* `web`（管理与自服务门户）：提供管理控制台及员工设备绑定、自助激活入口。
* `management`（模块化控制面）：负责身份、设备、资源、策略、IPAM、隧道期望配置和审计。
* `signal`（南向信令）：维护 Agent 双向流、接收状态并传递控制指令；可按部署需求独立运行。
* `cmd/cli`（管理命令行）：提供面向管理员和运维人员的命令行入口。

## 4.2 数据面应用 (Data Plane)

* `client`（终端 Agent）：跨平台运行，负责设备状态、WireGuard 隧道、Split DNS、进程特征和本机规则执行。
* `gateway`（边界网关 Agent）：部署于企业网络边界，负责 WireGuard Peer、路由、地址映射和网关访问规则。
* `l7proxy`（应用层审计代理）：检查网关转交的 HTTP、MySQL 等协议流量并输出审计事件。
* `relay`（中继服务）：按需提供无法直连时的数据转发能力。

------------------------------
## 5. 单仓库工程结构设计与目录树
项目采用单一 `velotrust/` Go monorepo，控制面初期作为模块化单体组织；Agent、信令、网关、七层代理及共享协议按模块边界划分。具体目录与职责见 [`ARCHITECTURE.md`](ARCHITECTURE.md)。
------------------------------
## 6. 全局南向双向流通信契约 (api.proto)
作为控制面与数据面连接的纽带，以下是驱动全网策略流、流量审计上报和动态漫游的底层 gRPC 核心契约：

syntax = "proto3";
package velotrust.v1;
option go_package = "://github.com";
service ControlPlaneService {
  rpc StreamConnect(stream AgentStatusReport) returns (stream ControlInstruction);
}
enum AgentType {
  AGENT_TYPE_UNSPECIFIED = 0;
  AGENT_TYPE_TERMINAL = 1;  
  AGENT_TYPE_GATEWAY = 2;   
}
message AgentStatusReport {
  string agent_id = 1;            
  AgentType type = 2;             
  string current_physical_ip = 3; 
  string local_session_token = 4; 
  DevicePosture posture = 5;      
  TrafficStats stats = 6;         
}
message DevicePosture {
  bool firewall_enabled = 1;      
  bool antivirus_running = 2;     
  string os_version = 3;          
  repeated string active_processes = 4; 
}
message TrafficStats {
  uint64 bytes_sent = 1;
  uint64 bytes_received = 2;
  uint32 active_connections = 3;
}
message ControlInstruction {
  string instruction_id = 1;
  int64 timestamp = 2;
  
  oneof payload {
    TunnelConfig tunnel_config = 3;     
    PolicyConfig policy_config = 4;     
    DnsConfig dns_config = 5;           
    DisconnectOrder disconnect = 6;     
  }
}
message TunnelConfig {
  string virtual_ip = 1;                
  string wg_private_key = 2;            
  uint32 listen_port = 3;               
  repeated WireGuardPeer peers = 4;     
  repeated NatMapping nat_mappings = 5; 
}
message WireGuardPeer {
  string public_key = 1;                
  string endpoint = 2;                  
  repeated string allowed_ips = 3;      
  uint32 persistent_keepalive = 4;      
}
message NatMapping {
  string shadow_cidr = 1;               
  string real_cidr = 2;                 
}
message PolicyConfig {
  repeated AclRule rules = 1;
  uint32 fail_safe_timeout_mins = 2;   
}
enum AclAction {
  ACL_ACTION_UNSPECIFIED = 0;
  ACL_ACTION_ALLOW = 1;
  ACL_ACTION_DENY = 2;
}
message AclRule {
  string rule_id = 1;
  string source_virtual_ip = 2;         
  string dest_ip_or_subnet = 3;         
  string protocol = 4;                  
  string dest_port = 5;                 
  AclAction action = 6;                 
  repeated string allowed_processes = 7;
}
message DnsConfig {
  repeated DnsZone zones = 1;
}
message DnsZone {
  string domain_suffix = 1;             
  repeated string dns_servers = 2;      
}
message DisconnectOrder {
  enum Reason {
    REASON_UNSPECIFIED = 0;
    REASON_SESSION_EXPIRED = 1;         
    REASON_POSTURE_FAILED = 2;          
    REASON_SECURITY_MELTDOWN = 3;       
  }
  Reason reason = 1;
  string message = 2;                   
}

------------------------------
## 7. 方案合规与开源借用策略限制 (AGPL-3.0 防御)
由于团队在开发中可能会参考开源 WireGuard 标杆项目（如 NetBird 或是 Headscale），本方案特制定严格的干净房间设计（Clean Room Design）隔离审查红线，以规避潜在的法律合规风险：

   1. 代码抄袭熔断规则：NetBird 控制面涉及的 /management 等核心微服务采用了具有强传染性的 AGPL-3.0 协议。本方案严禁直接进行任何代码段的 Copy-Paste 引入，防止 VeloTrust 被动沦为开源项目。
   2. 合理吸纳与黑盒依赖：开发团队仅允许将成熟的、采用宽松开源协议（如 MIT、Apache-2.0、BSD）的基础网络组件包（例如 wireguard-go 核心驱动、vishvananda/netlink 路由通信库等）作为第三方公开依赖引入工程。
   3. 技术重写规范：对于涉及 Linux 底层配置、DNS 拦截处理等技术路径，开发团队应理解其实现机制，再根据 VeloTrust 的多云集中式资源控制链条，在 monorepo 对应模块中独立编写和维护实现，确立清晰的知识产权边界。

------------------------------


