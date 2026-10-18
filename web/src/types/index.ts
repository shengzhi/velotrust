export type NodeStatus = 'online' | 'offline' | 'degraded' | 'pending_enrollment';
export type NodeType = 'gateway' | 'client';

export interface Node {
  id: string;
  name: string;
  type: NodeType;
  nodeTypeLabel?: string; // e.g. "站点接入网关", "服务代理网关", "边缘中继网关"
  virtualIp: string;
  siteId: string;
  siteName: string;
  networkName?: string; // e.g. "上海生产网", "北京核心网"
  networkId?: string;
  allowedSubnets?: string[]; // e.g. ["10.20.0.0/16"]
  nodeRoles?: string[]; // e.g. ["流量转发", "站点互联"]
  registrationMethod?: 'token' | 'manual' | 'cloud_init'; // e.g. "token" (一次性注册凭证)
  requireApproval?: boolean; // 是否需要审批
  enrollmentStatus?: 'unregistered' | 'registered' | 'pending_approval';
  enrollmentTokenId?: string;
  status: NodeStatus;
  lastHandshake: string; // ISO date string
  rxBytes: number;
  txBytes: number;
  endpoint: string;
  publicKey: string;
  createdAt: string;
}

export interface Peer {
  id: string;
  nodeId: string;
  peerNodeId: string;
  peerName: string;
  virtualIp: string;
  status: NodeStatus;
  lastHandshake: string;
  rxBytes: number;
  txBytes: number;
  endpoint: string;
  latencyMs: number;
}

export interface AclRule {
  id: string;
  nodeId?: string;
  direction: 'ingress' | 'egress';
  source: string;
  destination: string;
  port: string;
  action: 'allow' | 'deny';
  priority: number;
  description?: string;
}

export interface User {
  id: string;
  username: string;
  name: string;
  email: string;
  department: string;
  roles: ('admin' | 'user' | 'auditor')[];
  groups: string[];
  status: 'active' | 'disabled';
  lastLogin: string;
  createdAt: string;
}

export interface Group {
  id: string;
  name: string;
  description: string;
  memberCount: number;
  memberIds: string[];
}

export type DeviceType = 'PC' | 'Phone' | 'Tablet' | 'Server';
export type DeviceStatus = 'active' | 'revoked' | 'offline';

export interface Device {
  id: string;
  name: string;
  userId: string;
  userName: string;
  type: DeviceType;
  os: string;
  virtualIp: string;
  status: DeviceStatus;
  lastSeen: string;
  siteId: string;
  siteName: string;
  token?: string;
  createdAt: string;
}

export interface VpcNatMapping {
  enabled: boolean;
  virtualCidr: string; // 映射后的虚拟影子网段, 如 "172.31.10.0/24"
  natType: '1:1_Bidirectional' | 'SNAT_Masquerade' | 'DNAT_VirtualIP';
  description?: string;
  mappedAt?: string;
}

export interface Site {
  id: string;
  name: string;
  localSubnet: string;
  gatewayNodeId: string;
  gatewayNodeName: string;
  nodeCount: number;
  status: 'active' | 'inactive';
  createdAt: string;
  // VPC 重叠网段 NAT 映射 (VPC Overlap NAT)
  natMapping?: VpcNatMapping;
  cloudProvider?: 'Aliyun' | 'TencentCloud' | 'AWS' | 'HuaweiCloud' | 'IDC_OnPrem';
}

export interface Policy {
  id: string;
  name: string;
  subject: string; // e.g. "group:devs", "site:office-a", "user:admin"
  resource: string; // e.g. "subnet:prod-db", "site:datacenter"
  targetIp: string;
  ports: string; // e.g. "80,443" or "22" or "*"
  action: 'allow' | 'deny';
  priority: number;
  enabled: boolean;
  createdAt: string;
  // 进程级应用网络防火墙 (App / Process-Level Split Tunneling)
  appConstraint?: {
    enabled: boolean;
    processNames: string[]; // e.g. ["DataGrip.exe", "Navicat.exe", "curl", "psql"]
    mode: 'allowlist' | 'blocklist';
    binarySignatureCheck?: boolean; // 校验应用指纹哈希
  };
}

export interface FailSafeConfig {
  heartbeatIntervalSec: number; // 心跳间隔, 默认 15s
  offlineThresholdSec: number; // 判定失联阈值, 默认 60s
  mode: 'ALLOW_EXISTING_CACHE' | 'BLOCK_ALL_EMERGENCY' | 'READ_ONLY_FALLBACK';
  localCacheTtlHours: number; // 本地策略缓存最长保留时间, 如 72h
  deadMansSwitchEnabled: boolean; // 死信定时器 (Dead Man's Switch)
  deadMansTimeoutMinutes: number; // 离线熔断阈值(分钟), 如 30分钟
  emergencyAdminBypass: boolean; // 是否允许离线紧急本地管理口
  lastDrillRun?: string; // 最近演练时间
  autoRecovery: boolean; // 控制面板恢复后自动全量重同步
  gatewaysInFailSafe?: number;
  lastSyncAt?: string;
}

export interface AuditRecord {
  id: string;
  timestamp: string;
  user: string;
  device: string;
  node: string;
  sourceIp: string;
  destIp: string;
  port: number;
  action: 'allow' | 'deny';
  bytes: number;
}

export interface DashboardStats {
  totalNodes: number;
  onlineNodes: number;
  offlineNodes: number;
  degradedNodes: number;
  totalUsers: number;
  totalDevices: number;
  totalSites: number;
  statusDistribution: {
    label: string;
    value: number;
    color: string;
  }[];
  trafficTrend: {
    time: string;
    rx: number;
    tx: number;
  }[];
  recentAudit: AuditRecord[];
}

export interface DnsRecord {
  id: string;
  type: 'A' | 'CNAME' | 'TXT';
  host: string;
  value: string;
  ttl?: number;
}

export interface DnsZone {
  id: string;
  zone_id: string;
  domain_suffix: string;
  description: string;
  dns_servers: string[];
  bind_gateway?: string;
  records: DnsRecord[];
  assigned_groups: string[];
  status: 'Active' | 'Paused';
  peer_count: number;
  createdAt: string;
}

export interface IdpGroupMapping {
  id: string;
  idp_group_name: string;
  velotrust_group_id: string;
}

export type IdpBrand = 'Feishu' | 'DingTalk' | 'Okta' | 'WeCom' | 'Custom';
export type IdpProtocolType = 'OIDC' | 'SAML2' | 'OAuth2';

export interface IdpProvider {
  id: string;
  provider_id: string;
  type: IdpBrand;
  provider_type: IdpProtocolType;
  name: string;
  issuer_url: string;
  client_id: string;
  client_secret: string;
  group_mapping: IdpGroupMapping[];
  session_ttl_hours: number;
  status: 'Connected' | 'Disconnected' | 'Syncing' | 'Error';
  sync_time: string;
  mfa_required: boolean;
  user_count: number;
}

export interface IpamPool {
  id: string;
  pool_name: string;
  cidr: string;
  type: 'Gateway' | 'User-Device' | 'Service-VIP';
  allocation_mode: 'static' | 'dynamic';
  idp_group?: string;
  gateway_binding?: string;
  used: number;
  total: number;
  description?: string;
  color: string;
  status: 'Active' | 'Full' | 'Reserved';
}

export interface IpamLease {
  id: string;
  virtual_ip: string;
  node_type: 'Gateway' | 'Peer' | 'Server';
  target_name: string;
  endpoint: string;
  allocation_mode: '自动化分配-固定' | '飞书触发-动态锁定' | '钉钉触发-动态锁定' | '管理员手动保留';
  pool_id: string;
  pool_name: string;
  status: 'Active' | 'Idle' | 'Reserved';
  wireguard_pubkey: string;
  last_handshake: string;
  allocated_at: string;
}

export interface OverlayNetwork {
  global_cidr: string;
  total_ips: number;
  used_ips: number;
  free_ips: number;
  ipam_pools: IpamPool[];
  active_leases: IpamLease[];
}

export interface IpAllocationRequest {
  target_name: string;
  node_type: 'Gateway' | 'Peer' | 'Server';
  pool_id: string;
  idp_group?: string;
  wireguard_pubkey?: string;
  endpoint?: string;
  notes?: string;
}

export interface IpAllocationResult {
  allocated_ip: string;
  lease: IpamLease;
  wg_interface_config: string;
  gateway_vip: string;
  dns_server: string;
}

// ---------------- RBAC & Role Management Types ----------------
export type SeparationRoleType = 'sys_admin' | 'sec_admin' | 'audit_admin' | 'super_admin' | 'custom' | 'employee';

export interface PermissionItem {
  id: string;
  code: string; // e.g. 'menu:nodes', 'action:node:create', 'action:policy:publish'
  name: string;
  description: string;
  category: 'menu' | 'action';
  module: 'overview' | 'network' | 'resource' | 'policy' | 'identity' | 'audit' | 'system';
}

export interface Role {
  id: string;
  name: string;
  code: string;
  type: 'system' | 'custom';
  separationType: SeparationRoleType;
  description: string;
  userCount: number;
  menuRoutes: string[]; // 菜单访问权限：允许访问的控制台页面/路由
  permissionCodes: string[]; // 资源操作权限：细粒度操作代码
  apiResourceCodes?: string[]; // 资源访问权限：允许调用的系统 API 接口资源
  dataScope?: 'all' | 'dept' | 'self' | 'custom'; // 资源访问权限：数据访问范围
  createdAt: string;
  updatedAt: string;
}

// ---------------- ZTNA Protected Business Resources Types ----------------
export type ResourceCategory = 'web_portal' | 'database' | 'server_ssh' | 'rdp_desktop' | 'api_gateway' | 'cloud_console';
export type ResourceProtocol = 'HTTPS' | 'HTTP' | 'SSH' | 'RDP' | 'MySQL' | 'PostgreSQL' | 'TCP' | 'UDP';
export type SensitivityLevel = 'Public' | 'Internal' | 'Confidential' | 'TopSecret';

export interface ProtectedResource {
  id: string;
  name: string;
  code: string;
  category: ResourceCategory;
  protocol: ResourceProtocol;
  host: string;
  port: number;
  url?: string;
  siteId: string;
  siteName: string;
  gatewayNodeId: string;
  gatewayNodeName: string;
  sensitivity: SensitivityLevel;
  department: string;
  owner: string;
  status: 'healthy' | 'degraded' | 'unreachable';
  latencyMs: number;
  healthCheckType: 'TCP_SYN' | 'HTTP_GET' | 'ICMP_PING';
  associatedPoliciesCount: number;
  tags: string[];
  description: string;
  createdAt: string;
}

// ---------------- 菜单管理数据模型 (Menu Management) ----------------
export type MenuType = 'directory' | 'menu' | 'button';

export interface SysMenu {
  id: string;
  parentId: string | null;
  title: string;
  path: string;
  component?: string;
  icon: string;
  sortOrder: number;
  type: MenuType;
  permissionCode: string;
  visible: boolean;
  status: 'active' | 'disabled';
  badge?: string;
  isExternal?: boolean;
  createdAt: string;
  children?: SysMenu[];
}

// ---------------- VeloTrust 权限模型核心抽象 (Resource -> Permission -> API/RPC Mapping) ----------------

export type ResourceActionIdentifier = 'read' | 'write' | 'execute' | 'manage' | string;

export interface AuthResourceAction {
  action: ResourceActionIdentifier; // 操作标识: read, write, execute, manage, or sensitive actions
  label: string; // 标识名称: 读取, 写入, 执行, 管理
  description: string; // 语义含义
  isSensitive?: boolean; // 是否属于密码重置、角色授权变更、密钥轮换等敏感高危操作
}

export interface AuthResource {
  id: string;
  code: string; // 唯一标识: user, role, audit, gateway, route, policy, secret, device, ipam
  name: string; // 名称: 用户管理, 角色管理, 审计日志等
  category: 'identity' | 'network' | 'audit' | 'security' | 'system'; // 业务分类
  supportedActions: AuthResourceAction[]; // 支持的操作类型列表
  description: string;
  status: 'active' | 'disabled';
  createdAt: string;
}

export type PermissionType = 'api' | 'business_asset' | 'service' | 'data';

export interface SysPermission {
  id: string;
  code: string; // 组合标识: user:read, user:write, role:read, gateway:execute
  name: string;
  type?: PermissionType; // 资源对象类型 (保留向前兼容)
  resourceCode: string; // 所属资源标识: user, role, audit, gateway 等
  resourceName: string; // 所属资源名称: 用户管理
  action: string; // 操作类型: read, write, execute, manage
  actionLabel?: string; // 操作名称: 读取, 写入, 执行, 管理
  isSensitive?: boolean; // 是否敏感权限
  module: 'overview' | 'network' | 'resource' | 'policy' | 'identity' | 'audit' | 'system' | 'security';
  resourceUri?: string;
  resourceMethod?: ApiRequestMethod;
  dataScope?: 'all' | 'dept' | 'self' | 'custom';
  description: string;
  status: 'active' | 'disabled';
  assignedRolesCount: number;
  assignedRoles?: string[];
  mappedApisCount?: number;
  createdAt: string;
}

// ---------------- API 与系统服务资源数据模型 (API & RPC Mapping Catalog) ----------------
export type ApiRequestMethod = 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH' | '*';
export type ResourceRiskLevel = 'Low' | 'Medium' | 'High' | 'Critical';

export interface ApiRpcMapping {
  id: string;
  code: string; // 接口标识 / RPC 方法: iam.user.GetUser, iam.user.CreateUser, net.gateway.SyncPeers
  name: string; // 接口名称: 获取用户详情
  type?: 'HTTP_API' | 'gRPC' | 'API' | 'DATA' | 'SERVICE' | 'RPC'; // 协议类型
  method: ApiRequestMethod; // 请求方法
  path?: string; // URL 路径 / gRPC 服务路径: /api/v1/users/:id
  url?: string; // 兼容老字段
  serviceModule: 'overview' | 'network' | 'resource' | 'policy' | 'identity' | 'audit' | 'system' | 'security';
  rateLimit?: string;
  authRequired: boolean; // 是否受保护需要授权 (true 必须映射有效权限)
  riskLevel: ResourceRiskLevel;
  mappedPermissionCode?: string | null; // 映射的目标权限标识: user:read (null 表示未映射)
  boundPermissionCode?: string; // 兼容老字段
  mappedResourceCode?: string | null; // 所属资源: user
  mappedAction?: string | null; // 所属操作: read
  status?: 'mapped' | 'unmapped' | 'conflict' | 'active' | 'deprecated' | 'disabled'; // 映射状态
  conflictReason?: string;
  description: string;
  createdAt: string;
}

// 兼容别名
export type ApiResource = ApiRpcMapping;

// ---------------- 登录日志数据模型 (Login & Auth Audit Logs) ----------------
export type LoginAuthType = 'password' | 'idp_feishu' | 'idp_dingtalk' | 'idp_okta' | 'mfa_totp' | 'wireguard_cert';
export type LoginStatus = 'success' | 'failed' | 'locked' | 'mfa_rejected' | 'risk_blocked';

export interface LoginLog {
  id: string;
  timestamp: string;
  username: string;
  name: string;
  loginType: LoginAuthType;
  status: LoginStatus;
  ip: string;
  location: string;
  device: string;
  browser: string;
  message: string;
  riskScore: number; // 0-100
  sessionId: string;
  mfaUsed: boolean;
}

// ---------------- 操作审计日志数据模型 (Operation Audit Logs) ----------------
export type OperationActionType =
  | 'CREATE'
  | 'UPDATE'
  | 'DELETE'
  | 'ENABLE'
  | 'DISABLE'
  | 'EXPORT'
  | 'GRANT'
  | 'POLICY_ENFORCE'
  | 'DRILL_TEST';

export type OperationStatus = 'SUCCESS' | 'FAILED' | 'BLOCKED';

export interface OperationLog {
  id: string;
  timestamp: string;
  operatorUsername: string; // 操作账号, e.g. "admin", "sec_officer", "net_architect"
  operatorName: string; // 姓名, e.g. "系统超管", "李安全", "张网络"
  operatorRole: string; // 角色, e.g. "超级管理员", "安全保密员", "网络管理员"
  module: string; // 业务功能模块, e.g. "访问控制策略", "角色与权限", "虚拟节点与网关", "企业资源管理", "系统用户管理", "系统容灾与设置"
  actionType: OperationActionType; // 动作类型
  actionName: string; // 操作行为概述
  targetResource: string; // 操作对象与标识
  requestMethod: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH';
  requestUri: string; // 请求URI
  ip: string; // 客户端 IP
  location: string; // 客户端地理位置
  device: string; // 操作终端环境
  status: OperationStatus; // 状态: SUCCESS, FAILED, BLOCKED
  durationMs: number; // 耗时
  errorMessage?: string; // 失败/拦截说明
  diffSnapshot?: {
    before?: Record<string, any>;
    after?: Record<string, any>;
  };
}

// ---------------- 一次性网关注册凭证模型 (Enrollment Token) ----------------
export type EnrollmentTokenStatus =
  | 'PENDING'   // 有效待注册 (Unused, within expiration)
  | 'USED'      // 已使用入网 (Consumed, immediately invalidated upon registration)
  | 'EXPIRED'   // 已过期 (Past expiresAt without registration)
  | 'REVOKED';  // 已作废 (Revoked by admin)

export interface EnrollmentToken {
  id: string; // e.g. "tok-wg-sh-prod-01"
  token: string; // e.g. "vt_tok_7a9f8e2c0d4b6a1e3f5..."
  nodeId: string; // 绑定的预创建逻辑节点 ID
  nodeName: string; // e.g. "shanghai-prod-gateway-01"
  nodeType: string; // e.g. "站点接入网关"
  networkName: string; // e.g. "上海生产网"
  networkId?: string;
  allowedSubnets: string[]; // e.g. ["10.20.0.0/16"]
  nodeRoles: string[]; // e.g. ["流量转发", "站点互联"]
  registrationMethod: 'token' | 'manual';
  requireApproval: boolean;
  approvalStatus?: 'approved' | 'pending_approval' | 'rejected';
  approvedBy?: string;
  approvedAt?: string;

  // 安全边界 (Zero-Trust Bounds)
  permissions: string[]; // 仅包含只读入网权限: ["node:register_only"]，严格无管理 API 或业务资源权限
  maxUsage: number; // 默认 1 (一次性)
  usedCount: number; // 当前使用次数 (0 或 1)

  status: EnrollmentTokenStatus;

  // 审计与时间
  createdAt: string;
  createdBy: string; // 管理员账号，如 "admin"
  expiresAt: string; // 明确过期时间
  usedAt?: string; // 消耗时间
  usedIp?: string; // 注册时的对端 IP
  deviceFingerprint?: string; // 硬件/操作系统特征
  revokeReason?: string;
  revokedAt?: string;
  revokedBy?: string;

  // 自动化入网引导
  installCommand: string;
  configSnippet: string;
  notes?: string;
}

export interface PreCreateNodeInput {
  name: string; // 节点名称，例如 shanghai-prod-gateway-01
  nodeTypeLabel: string; // 节点类型，例如 站点接入网关
  networkName: string; // 所属网络，例如 上海生产网
  allowedSubnets: string[]; // 允许访问的网段，例如 ["10.20.0.0/16"]
  nodeRoles: string[]; // 节点角色，例如 ["流量转发", "站点互联"]
  registrationMethod: 'token'; // 注册方式: 一次性注册凭证
  requireApproval: boolean; // 是否需要审批: 是/否
  siteId?: string;
  siteName?: string;
  virtualIp?: string;
}

export interface GenerateTokenInput {
  nodeId: string;
  expiresInHours: number; // 过期时间 (小时)，如 1, 24, 72, 168
  notes?: string;
}


