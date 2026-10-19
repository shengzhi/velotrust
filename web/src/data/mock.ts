import {
  Node,
  Peer,
  AclRule,
  User,
  Group,
  Device,
  Site,
  Policy,
  AuditRecord,
  DashboardStats,
  DnsZone,
  DnsRecord,
  IdpProvider,
  IdpGroupMapping,
  IpamPool,
  IpamLease,
  OverlayNetwork,
  FailSafeConfig,
  Role,
  PermissionItem,
  ProtectedResource,
  SysMenu,
  SysPermission,
  ApiResource,
  ApiRpcMapping,
  AuthResource,
  LoginLog,
  OperationLog,
  EnrollmentToken,
  EnrollmentTokenStatus,
  PreCreateNodeInput,
  GenerateTokenInput,
} from '../types';
import {
  allPermissions,
  initialRoles,
  initialProtectedResources,
} from './rolesAndResourcesData';
import {
  initialSysMenus,
  initialAuthResources,
  initialSysPermissions,
  initialApiRpcMappings,
  initialApiResources,
  initialLoginLogs,
  initialOperationLogs,
} from './rbacCatalogData';

export {
  allPermissions,
  initialRoles,
  initialProtectedResources,
  initialSysMenus,
  initialSysPermissions,
  initialApiResources,
  initialLoginLogs,
  initialOperationLogs,
};


// Initial Mock Nodes
export const initialNodes: Node[] = [
  {
    id: 'node-sh-prod-gw-01',
    name: 'shanghai-prod-gateway-01',
    type: 'gateway',
    nodeTypeLabel: '站点接入网关',
    virtualIp: '10.0.0.10/24',
    siteId: 'site-sh',
    siteName: '上海研发中心',
    networkName: '上海生产网',
    allowedSubnets: ['10.20.0.0/16'],
    nodeRoles: ['流量转发', '站点互联'],
    registrationMethod: 'token',
    requireApproval: true,
    enrollmentStatus: 'unregistered',
    status: 'pending_enrollment',
    lastHandshake: new Date(Date.now() - 1000 * 3600 * 2).toISOString(),
    rxBytes: 0,
    txBytes: 0,
    endpoint: '等待节点上报 (待注册)',
    publicKey: 'PENDING_REGISTRATION_KEY',
    createdAt: new Date(Date.now() - 1000 * 3600 * 2).toISOString(),
  },
  {
    id: 'node-wh-iot-gw-02',
    name: 'wuhan-iot-gateway-02',
    type: 'gateway',
    nodeTypeLabel: '边缘中继网关 (Relay)',
    virtualIp: '10.0.0.12/24',
    siteId: 'site-sz',
    siteName: '武汉物联节点',
    networkName: '武汉车联网测试区',
    allowedSubnets: ['192.168.100.0/24'],
    nodeRoles: ['流量转发'],
    registrationMethod: 'token',
    requireApproval: false,
    enrollmentStatus: 'unregistered',
    status: 'pending_enrollment',
    lastHandshake: new Date(Date.now() - 1000 * 3600 * 6).toISOString(),
    rxBytes: 0,
    txBytes: 0,
    endpoint: '等待节点上报 (未生成凭证)',
    publicKey: 'PENDING_REGISTRATION_KEY',
    createdAt: new Date(Date.now() - 1000 * 3600 * 6).toISOString(),
  },
  {
    id: 'node-hub-bj',
    name: 'gw-bj-hub-01',
    type: 'gateway',
    nodeTypeLabel: '核心中枢网关 (Hub)',
    networkName: '北京核心生产网',
    allowedSubnets: ['10.0.0.0/16', '10.100.0.0/16'],
    nodeRoles: ['流量转发', '站点互联'],
    registrationMethod: 'token',
    requireApproval: false,
    enrollmentStatus: 'registered',
    virtualIp: '10.0.0.1/24',
    siteId: 'site-bj',
    siteName: '北京总部核心区',
    status: 'online',
    lastHandshake: new Date(Date.now() - 1000 * 12).toISOString(),
    rxBytes: 154289201948,
    txBytes: 198302194812,
    endpoint: '202.108.22.5:51820',
    publicKey: 'aJ9uQ0kL8vM7nW2xY1zP4qR3sT6uV5wX8yZ0aB1cD2e=',
    createdAt: '2024-01-10T08:00:00Z',
  },
  {
    id: 'node-gw-sh',
    name: 'gw-sh-branch-01',
    type: 'gateway',
    nodeTypeLabel: '站点接入网关',
    networkName: '上海研发混合云网',
    allowedSubnets: ['192.168.10.0/24', '172.31.10.0/24'],
    nodeRoles: ['流量转发', '跨云NAT转换'],
    virtualIp: '10.0.0.2/24',
    siteId: 'site-sh',
    siteName: '上海研发中心',
    status: 'online',
    lastHandshake: new Date(Date.now() - 1000 * 25).toISOString(),
    rxBytes: 84920194812,
    txBytes: 67104819204,
    endpoint: '139.196.88.14:51820',
    publicKey: 'bK0vR1lM9wN8oX3yZ2aQ5rS4tU7vW6xY9zA1bC2dE3f=',
    createdAt: '2024-01-12T09:30:00Z',
  },
  {
    id: 'node-gw-sz',
    name: 'gw-sz-branch-01',
    type: 'gateway',
    nodeTypeLabel: 'IDC机房接入网关',
    networkName: '深圳创新网络站点',
    allowedSubnets: ['192.168.30.0/24'],
    nodeRoles: ['流量转发', '物理机房互联'],
    virtualIp: '10.0.0.3/24',
    siteId: 'site-sz',
    siteName: '深圳创新基地',
    status: 'online',
    lastHandshake: new Date(Date.now() - 1000 * 45).toISOString(),
    rxBytes: 61840192841,
    txBytes: 52918401928,
    endpoint: '119.29.35.80:51820',
    publicKey: 'cL1wS2mN0xO9pY4zA3bR6sT5uV8wX7yZ0aB2cD3eF4g=',
    createdAt: '2024-01-15T10:15:00Z',
  },
  {
    id: 'node-gw-fra',
    name: 'gw-fra-cloud-01',
    type: 'gateway',
    nodeTypeLabel: '跨国灾备中继网关 (Relay)',
    networkName: 'AWS 欧洲灾备网',
    allowedSubnets: ['10.100.0.0/20'],
    nodeRoles: ['容灾中继', '跨国同步'],
    virtualIp: '10.0.0.4/24',
    siteId: 'site-fra',
    siteName: '法兰克福灾备中心',
    status: 'degraded',
    lastHandshake: new Date(Date.now() - 1000 * 180).toISOString(),
    rxBytes: 24810294810,
    txBytes: 18491029481,
    endpoint: '185.12.64.92:51820',
    publicKey: 'dM2xT3nO1yP0qZ5aB4cS7tU6vW9xY8zA1bC3dE4fG5h=',
    createdAt: '2024-02-01T14:20:00Z',
  },
  {
    id: 'node-gw-us',
    name: 'gw-us-cloud-01',
    type: 'gateway',
    nodeTypeLabel: '海外分支网关',
    networkName: '北美分支办公网',
    allowedSubnets: ['172.16.0.0/20'],
    nodeRoles: ['分支接入'],
    virtualIp: '10.0.0.5/24',
    siteId: 'site-us',
    siteName: '硅谷海外分部',
    status: 'offline',
    lastHandshake: new Date(Date.now() - 1000 * 3600 * 4).toISOString(),
    rxBytes: 12049182041,
    txBytes: 8192049182,
    endpoint: '54.218.102.44:51820',
    publicKey: 'eN3yU4oP2zQ1ra6bC5dT8uV7wX0yZ9aB2cD4eF5gH6i=',
    createdAt: '2024-02-10T16:00:00Z',
  },
  {
    id: 'node-client-mbp-dev',
    name: 'dev-macbook-pro16',
    type: 'client',
    virtualIp: '10.0.1.101/24',
    siteId: 'site-sh',
    siteName: '上海研发中心',
    status: 'online',
    lastHandshake: new Date(Date.now() - 1000 * 60).toISOString(),
    rxBytes: 4291048192,
    txBytes: 1849102948,
    endpoint: '180.168.41.22:60114',
    publicKey: 'fO4zV5pQ3aR2sb7cD6eU9vW8xY1zA0bC3dE5fG6hI7j=',
    createdAt: '2024-03-05T09:00:00Z',
  },
  {
    id: 'node-client-ops-tp',
    name: 'ops-thinkpad-x1',
    type: 'client',
    virtualIp: '10.0.1.102/24',
    siteId: 'site-bj',
    siteName: '北京总部核心区',
    status: 'online',
    lastHandshake: new Date(Date.now() - 1000 * 85).toISOString(),
    rxBytes: 8941029481,
    txBytes: 3104918204,
    endpoint: '124.65.178.6:58921',
    publicKey: 'gP5aW6qR4bS3tc8dE7fV0wX9yZ2aB1cD4eF6gH7iJ8k=',
    createdAt: '2024-03-08T11:20:00Z',
  },
  {
    id: 'node-client-ci-runner',
    name: 'ci-runner-ubuntu-01',
    type: 'client',
    virtualIp: '10.0.1.103/24',
    siteId: 'site-bj',
    siteName: '北京总部核心区',
    status: 'online',
    lastHandshake: new Date(Date.now() - 1000 * 30).toISOString(),
    rxBytes: 45910294812,
    txBytes: 88194019284,
    endpoint: '202.108.22.18:54120',
    publicKey: 'hQ6bX7rS5cT4ud9eF8gW1xY0zA3bC2dE5fG7hI8jK9l=',
    createdAt: '2024-03-15T15:40:00Z',
  },
  {
    id: 'node-client-db-backup',
    name: 'db-replica-sync-01',
    type: 'client',
    virtualIp: '10.0.1.104/24',
    siteId: 'site-sz',
    siteName: '深圳创新基地',
    status: 'degraded',
    lastHandshake: new Date(Date.now() - 1000 * 320).toISOString(),
    rxBytes: 78910294812,
    txBytes: 4191029481,
    endpoint: '119.29.35.95:52310',
    publicKey: 'iR7cY8sT6dU5ve0fG9hX2yZ1aB4cD3eF6gH8iJ9kL0m=',
    createdAt: '2024-04-01T08:30:00Z',
  },
  {
    id: 'node-client-ceo-ipad',
    name: 'executive-ipad-pro',
    type: 'client',
    virtualIp: '10.0.1.105/24',
    siteId: 'site-bj',
    siteName: '北京总部核心区',
    status: 'offline',
    lastHandshake: new Date(Date.now() - 1000 * 3600 * 8).toISOString(),
    rxBytes: 1294810294,
    txBytes: 310294810,
    endpoint: '124.65.178.99:62100',
    publicKey: 'jS8dZ9tU7eV6wf1gH0iY3zA2bC5dE4fG7hI9jK0lM1n=',
    createdAt: '2024-04-12T17:10:00Z',
  },
  {
    id: 'node-client-sec-audit',
    name: 'sec-scanner-worker',
    type: 'client',
    virtualIp: '10.0.1.106/24',
    siteId: 'site-sh',
    siteName: '上海研发中心',
    status: 'online',
    lastHandshake: new Date(Date.now() - 1000 * 40).toISOString(),
    rxBytes: 19481029481,
    txBytes: 2591029481,
    endpoint: '139.196.88.42:57800',
    publicKey: 'kT9ea0uV8fW7xg2hI1jZ4aB3cD6eF5gH8iJ0kL1mN2o=',
    createdAt: '2024-04-20T10:00:00Z',
  },
];

// Initial Users
export const initialUsers: User[] = [
  {
    id: 'user-01',
    username: 'admin',
    name: '系统管理员 (张伟)',
    email: 'admin@mesh.internal',
    department: '网络基础设施部',
    roles: ['admin'],
    groups: ['安全运维组', '研发核心组'],
    status: 'active',
    lastLogin: new Date(Date.now() - 1000 * 60 * 5).toISOString(),
    createdAt: '2024-01-01T00:00:00Z',
  },
  {
    id: 'user-02',
    username: 'lina_dev',
    name: '李娜',
    email: 'lina.zhang@mesh.internal',
    department: '基础平台架构组',
    roles: ['user'],
    groups: ['研发核心组'],
    status: 'active',
    lastLogin: new Date(Date.now() - 1000 * 60 * 35).toISOString(),
    createdAt: '2024-01-15T09:00:00Z',
  },
  {
    id: 'user-03',
    username: 'wanglei_sec',
    name: '王磊',
    email: 'lei.wang@mesh.internal',
    department: '信息安全合规部',
    roles: ['admin', 'auditor'],
    groups: ['安全运维组', '第三方审计组'],
    status: 'active',
    lastLogin: new Date(Date.now() - 1000 * 3600 * 2).toISOString(),
    createdAt: '2024-01-20T11:00:00Z',
  },
  {
    id: 'user-04',
    username: 'liuyang_ops',
    name: '刘洋',
    email: 'yang.liu@mesh.internal',
    department: 'SRE可靠性工程组',
    roles: ['user'],
    groups: ['安全运维组'],
    status: 'active',
    lastLogin: new Date(Date.now() - 1000 * 3600 * 12).toISOString(),
    createdAt: '2024-02-01T14:00:00Z',
  },
  {
    id: 'user-05',
    username: 'chenmin_qa',
    name: '陈敏',
    email: 'min.chen@mesh.internal',
    department: '质量测试与自动化',
    roles: ['user'],
    groups: ['产品测试组'],
    status: 'active',
    lastLogin: new Date(Date.now() - 1000 * 3600 * 28).toISOString(),
    createdAt: '2024-02-15T16:20:00Z',
  },
  {
    id: 'user-06',
    username: 'zhaoqiang_aud',
    name: '赵强 (外聘合规)',
    email: 'qiang.zhao@external-audit.com',
    department: '外部信息安全评估中心',
    roles: ['auditor'],
    groups: ['第三方审计组'],
    status: 'disabled',
    lastLogin: new Date(Date.now() - 1000 * 86400 * 15).toISOString(),
    createdAt: '2024-03-01T08:00:00Z',
  },
];

// Initial Groups
export const initialGroups: Group[] = [
  {
    id: 'group-01',
    name: '安全运维组',
    description: '负责生产网络全网路由拓扑、访问策略、防火墙与堡垒机接入',
    memberCount: 3,
    memberIds: ['user-01', 'user-03', 'user-04'],
  },
  {
    id: 'group-02',
    name: '研发核心组',
    description: '研发人员专享开发网段接入、CI/CD编译机通信及预发测试集群',
    memberCount: 2,
    memberIds: ['user-01', 'user-02'],
  },
  {
    id: 'group-03',
    name: '第三方审计组',
    description: '具备只读审计日志访问权限，仅限于安全溯源与合规检查',
    memberCount: 2,
    memberIds: ['user-03', 'user-06'],
  },
  {
    id: 'group-04',
    name: '产品测试组',
    description: 'QA团队测试终端与自动化沙箱测试网段接入组',
    memberCount: 1,
    memberIds: ['user-05'],
  },
];

// Initial Devices
export const initialDevices: Device[] = [
  {
    id: 'dev-01',
    name: '张伟-MacBook-M3Max',
    userId: 'user-01',
    userName: '张伟',
    type: 'PC',
    os: 'macOS Sonoma 14.5',
    virtualIp: '10.0.1.101',
    status: 'active',
    lastSeen: new Date(Date.now() - 1000 * 60).toISOString(),
    siteId: 'site-bj',
    siteName: '北京总部核心区',
    token: 'wg-tok-98f2b71e-089c-4821-b384-219ad8f61a01',
    createdAt: '2024-01-15T09:00:00Z',
  },
  {
    id: 'dev-02',
    name: '李娜-ThinkPad-T14',
    userId: 'user-02',
    userName: '李娜',
    type: 'PC',
    os: 'Ubuntu 24.04 LTS',
    virtualIp: '10.0.1.102',
    status: 'active',
    lastSeen: new Date(Date.now() - 1000 * 60 * 20).toISOString(),
    siteId: 'site-sh',
    siteName: '上海研发中心',
    token: 'wg-tok-67a1c93f-5481-432d-9481-8172dfa52b02',
    createdAt: '2024-01-18T10:30:00Z',
  },
  {
    id: 'dev-03',
    name: '王磊-iPhone15-Pro',
    userId: 'user-03',
    userName: '王磊',
    type: 'Phone',
    os: 'iOS 18.1',
    virtualIp: '10.0.1.105',
    status: 'active',
    lastSeen: new Date(Date.now() - 1000 * 60 * 45).toISOString(),
    siteId: 'site-bj',
    siteName: '北京总部核心区',
    token: 'wg-tok-4389df01-bc81-4281-a981-91823719bc03',
    createdAt: '2024-01-22T14:10:00Z',
  },
  {
    id: 'dev-04',
    name: '刘洋-Dell-Precision',
    userId: 'user-04',
    userName: '刘洋',
    type: 'PC',
    os: 'Windows 11 Enterprise',
    virtualIp: '10.0.1.108',
    status: 'active',
    lastSeen: new Date(Date.now() - 1000 * 3600 * 3).toISOString(),
    siteId: 'site-sz',
    siteName: '深圳创新基地',
    token: 'wg-tok-19827364-ff88-4122-8172-bb8172635404',
    createdAt: '2024-02-05T11:00:00Z',
  },
  {
    id: 'dev-05',
    name: '李娜-iPad-Air5',
    userId: 'user-02',
    userName: '李娜',
    type: 'Tablet',
    os: 'iPadOS 17.6',
    virtualIp: '10.0.1.112',
    status: 'offline',
    lastSeen: new Date(Date.now() - 1000 * 3600 * 18).toISOString(),
    siteId: 'site-sh',
    siteName: '上海研发中心',
    token: 'wg-tok-99182736-4182-4aa1-9182-771829384705',
    createdAt: '2024-02-20T16:00:00Z',
  },
  {
    id: 'dev-06',
    name: '陈敏-Galaxy-S24',
    userId: 'user-05',
    userName: '陈敏',
    type: 'Phone',
    os: 'Android 15',
    virtualIp: '10.0.1.115',
    status: 'active',
    lastSeen: new Date(Date.now() - 1000 * 60 * 10).toISOString(),
    siteId: 'site-bj',
    siteName: '北京总部核心区',
    token: 'wg-tok-28374619-bb19-4821-8172-cc9182736406',
    createdAt: '2024-03-01T09:30:00Z',
  },
  {
    id: 'dev-07',
    name: '赵强-外协笔记本',
    userId: 'user-06',
    userName: '赵强 (外聘合规)',
    type: 'PC',
    os: 'Windows 10 Pro',
    virtualIp: '10.0.1.120',
    status: 'revoked',
    lastSeen: new Date(Date.now() - 1000 * 86400 * 16).toISOString(),
    siteId: 'site-bj',
    siteName: '北京总部核心区',
    token: 'wg-tok-revoked-81726354-9182',
    createdAt: '2024-03-05T14:00:00Z',
  },
];

// Initial Sites
export const initialSites: Site[] = [
  {
    id: 'site-bj',
    name: '北京总部核心区 (阿里云 VPC)',
    localSubnet: '192.168.10.0/24',
    gatewayNodeId: 'node-hub-bj',
    gatewayNodeName: 'gw-bj-hub-01',
    nodeCount: 5,
    status: 'active',
    cloudProvider: 'Aliyun',
    createdAt: '2024-01-01T00:00:00Z',
    natMapping: {
      enabled: false,
      virtualCidr: '192.168.10.0/24',
      natType: '1:1_Bidirectional',
      description: '标准 VPC 原生子网（作为基准无冲突）',
    },
  },
  {
    id: 'site-sh',
    name: '上海研发中心 (腾讯云 VPC)',
    localSubnet: '192.168.10.0/24', // 故意重叠示例：与北京阿里云同为 192.168.10.0/24!
    gatewayNodeId: 'node-gw-sh',
    gatewayNodeName: 'gw-sh-branch-01',
    nodeCount: 3,
    status: 'active',
    cloudProvider: 'TencentCloud',
    createdAt: '2024-01-10T00:00:00Z',
    natMapping: {
      enabled: true,
      virtualCidr: '172.31.10.0/24', // 映射为 172.31.10.0/24，完美规避重叠
      natType: '1:1_Bidirectional',
      description: '跨云网段重叠双向 1:1 NAT (192.168.10.x ➔ 172.31.10.x)',
      mappedAt: '2024-01-12T14:30:00Z',
    },
  },
  {
    id: 'site-sz',
    name: '深圳创新基地 (IDC 网络站点)',
    localSubnet: '192.168.30.0/24',
    gatewayNodeId: 'node-gw-sz',
    gatewayNodeName: 'gw-sz-branch-01',
    nodeCount: 2,
    status: 'active',
    cloudProvider: 'IDC_OnPrem',
    createdAt: '2024-01-15T00:00:00Z',
    natMapping: {
      enabled: false,
      virtualCidr: '192.168.30.0/24',
      natType: 'SNAT_Masquerade',
      description: 'IDC 物理网段直通',
    },
  },
  {
    id: 'site-fra',
    name: '法兰克福灾备中心 (AWS eu-central-1)',
    localSubnet: '10.100.0.0/20',
    gatewayNodeId: 'node-gw-fra',
    gatewayNodeName: 'gw-fra-cloud-01',
    nodeCount: 1,
    status: 'active',
    cloudProvider: 'AWS',
    createdAt: '2024-02-01T00:00:00Z',
    natMapping: {
      enabled: false,
      virtualCidr: '10.100.0.0/20',
      natType: '1:1_Bidirectional',
    },
  },
  {
    id: 'site-us',
    name: '硅谷海外分部 (华为云海外区)',
    localSubnet: '10.200.0.0/20',
    gatewayNodeId: 'node-gw-us',
    gatewayNodeName: 'gw-us-cloud-01',
    nodeCount: 1,
    status: 'inactive',
    cloudProvider: 'HuaweiCloud',
    createdAt: '2024-02-10T00:00:00Z',
  },
];

// Initial Policies
export const initialPolicies: Policy[] = [
  {
    id: 'pol-01',
    name: '默认跨站点安全互通',
    subject: 'site:*',
    resource: 'site:*',
    targetIp: '10.0.0.0/16',
    ports: '80,443,51820',
    action: 'allow',
    priority: 100,
    enabled: true,
    createdAt: '2024-01-01T00:00:00Z',
    appConstraint: {
      enabled: false,
      processNames: [],
      mode: 'allowlist',
    },
  },
  {
    id: 'pol-02',
    name: '生产数据库专网只允许特定客户端工具接入',
    subject: 'group:安全运维组',
    resource: 'subnet:prod-db-cluster',
    targetIp: '192.168.10.50/32',
    ports: '3306,5432,6379',
    action: 'allow',
    priority: 200,
    enabled: true,
    createdAt: '2024-01-10T09:00:00Z',
    appConstraint: {
      enabled: true,
      processNames: ['DataGrip.exe', 'Navicat.exe', 'DBeaver.exe', 'psql'],
      mode: 'allowlist',
      binarySignatureCheck: true,
    },
  },
  {
    id: 'pol-03',
    name: '全网禁用明文 Telnet 与非标管理端口',
    subject: '*',
    resource: '*',
    targetIp: '0.0.0.0/0',
    ports: '23,21,8080',
    action: 'deny',
    priority: 10,
    enabled: true,
    createdAt: '2024-01-15T12:00:00Z',
  },
  {
    id: 'pol-04',
    name: '研发组访问上海及北京测试集群 (限授信 IDE / 终端)',
    subject: 'group:研发核心组',
    resource: 'site:上海研发中心',
    targetIp: '192.168.20.0/24',
    ports: '22,80,443,8000-9000',
    action: 'allow',
    priority: 300,
    enabled: true,
    createdAt: '2024-02-01T15:30:00Z',
    appConstraint: {
      enabled: true,
      processNames: ['code.exe', 'idea64.exe', 'iterm2', 'ssh.exe', 'curl'],
      mode: 'allowlist',
      binarySignatureCheck: false,
    },
  },
  {
    id: 'pol-05',
    name: '离岸海外节点访问核心资产限流与拦截',
    subject: 'site:硅谷海外分部',
    resource: 'subnet:北京生产核心库',
    targetIp: '192.168.10.0/24',
    ports: '*',
    action: 'deny',
    priority: 50,
    enabled: false,
    createdAt: '2024-02-15T10:00:00Z',
  },
  {
    id: 'pol-06',
    name: '审计人员只读访问监控指标与日志平台',
    subject: 'group:第三方审计组',
    resource: 'subnet:elk-kibana-service',
    targetIp: '192.168.10.88/32',
    ports: '5601,9200',
    action: 'allow',
    priority: 250,
    enabled: true,
    createdAt: '2024-03-01T08:00:00Z',
  },
];

// Initial Mock DNS Zones (Split DNS)
export const initialDnsZones: DnsZone[] = [
  {
    id: 'dns-1',
    zone_id: 'dns_zone_01',
    domain_suffix: 'aliyun.internal',
    description: '阿里云内网测试环境集群域名解析',
    dns_servers: ['10.88.0.1'],
    bind_gateway: '阿里云华东网关 (gw-sh-branch-01)',
    records: [
      { id: 'rec-1', type: 'A', host: 'web', value: '192.168.10.5', ttl: 300 },
      { id: 'rec-2', type: 'A', host: 'mysql', value: '192.168.10.11', ttl: 300 },
      { id: 'rec-3', type: 'A', host: 'redis', value: '192.168.10.22', ttl: 300 },
    ],
    assigned_groups: ['研发核心组', '安全运维组'],
    status: 'Active',
    peer_count: 42,
    createdAt: '2024-02-01T08:00:00Z',
  },
  {
    id: 'dns-2',
    zone_id: 'dns_zone_02',
    domain_suffix: 'corp.internal',
    description: '北京总部生产内网代码仓库与业务中台',
    dns_servers: ['10.0.0.1'],
    bind_gateway: '北京核心网关 (gw-bj-hub-01)',
    records: [
      { id: 'rec-4', type: 'A', host: 'git', value: '10.0.1.103', ttl: 60 },
      { id: 'rec-5', type: 'CNAME', host: 'ci', value: 'git.corp.internal', ttl: 300 },
      { id: 'rec-6', type: 'A', host: 'wiki', value: '192.168.10.88', ttl: 300 },
      { id: 'rec-7', type: 'A', host: 'auth', value: '10.0.0.1', ttl: 300 },
    ],
    assigned_groups: ['安全运维组', '研发核心组', '产品测试组'],
    status: 'Active',
    peer_count: 68,
    createdAt: '2024-02-10T10:00:00Z',
  },
  {
    id: 'dns-3',
    zone_id: 'dns_zone_03',
    domain_suffix: 'aws.internal',
    description: '海外法兰克福灾备节点内网对象存储解析',
    dns_servers: ['10.0.0.4'],
    bind_gateway: '法兰克福灾备中心 (gw-fra-cloud-01)',
    records: [
      { id: 'rec-8', type: 'A', host: 'backup-s3', value: '10.100.0.12', ttl: 300 },
      { id: 'rec-9', type: 'A', host: 'es-cluster', value: '10.100.0.18', ttl: 300 },
    ],
    assigned_groups: ['安全运维组'],
    status: 'Active',
    peer_count: 15,
    createdAt: '2024-03-01T14:00:00Z',
  },
];

// Initial Mock IdP Providers (OIDC / OAuth2 / SAML)
export const initialIdpProviders: IdpProvider[] = [
  {
    id: 'idp-1',
    provider_id: 'idp_feishu_01',
    type: 'Feishu',
    provider_type: 'OIDC',
    name: '公司官方飞书认证源',
    issuer_url: 'https://open.feishu.cn/open-apis/authen/v1',
    client_id: 'cli_a123bc456def789',
    client_secret: 'ENC_KMS_9a8f7b6c5d4e3f2a1b0c',
    group_mapping: [
      { id: 'gm-1', idp_group_name: '基础架构部/后端开发', velotrust_group_id: '研发核心组' },
      { id: 'gm-2', idp_group_name: '信息安全部/安全运维', velotrust_group_id: '安全运维组' },
      { id: 'gm-3', idp_group_name: '质量保障与测试中心', velotrust_group_id: '产品测试组' },
      { id: 'gm-4', idp_group_name: '外部合规评估专家组', velotrust_group_id: '第三方审计组' },
    ],
    session_ttl_hours: 12,
    status: 'Connected',
    sync_time: '2026-10-12 16:30:00',
    mfa_required: true,
    user_count: 148,
  },
  {
    id: 'idp-2',
    provider_id: 'idp_dingtalk_02',
    type: 'DingTalk',
    provider_type: 'OAuth2',
    name: '钉钉企业认证网关 (预备源)',
    issuer_url: 'https://oapi.dingtalk.com/connect/oauth2/sns_authorize',
    client_id: 'ding_sec_8849201948',
    client_secret: 'ENC_KMS_3c2b1a0f9e8d7c6b5a4',
    group_mapping: [
      { id: 'gm-5', idp_group_name: '数字化运营部/运维协同', velotrust_group_id: '安全运维组' },
    ],
    session_ttl_hours: 8,
    status: 'Disconnected',
    sync_time: '2026-10-10 09:15:00',
    mfa_required: false,
    user_count: 36,
  },
  {
    id: 'idp-3',
    provider_id: 'idp_okta_03',
    type: 'Okta',
    provider_type: 'OIDC',
    name: '海外合规 Okta 统一身份域',
    issuer_url: 'https://velotrust.okta.com/oauth2/default',
    client_id: '0oa92kdf8103nflq0294',
    client_secret: 'ENC_KMS_7f8a9b0c1d2e3f4a5b6c',
    group_mapping: [
      { id: 'gm-6', idp_group_name: 'Global-SRE-Core', velotrust_group_id: '安全运维组' },
      { id: 'gm-7', idp_group_name: 'Engineering-Backend', velotrust_group_id: '研发核心组' },
    ],
    session_ttl_hours: 24,
    status: 'Connected',
    sync_time: '2026-10-12 15:45:00',
    mfa_required: true,
    user_count: 82,
  },
  {
    id: 'idp-4',
    provider_id: 'idp_wecom_04',
    type: 'WeCom',
    provider_type: 'OAuth2',
    name: '企业微信移动工作台扫码源',
    issuer_url: 'https://qyapi.weixin.qq.com/cgi-bin/sso',
    client_id: 'ww94810294810294',
    client_secret: 'ENC_KMS_wecom_sec_99182',
    group_mapping: [],
    session_ttl_hours: 12,
    status: 'Disconnected',
    sync_time: '未同步',
    mfa_required: false,
    user_count: 0,
  },
];

// Initial Mock IPAM Pools
export const initialIpamPools: IpamPool[] = [
  {
    id: 'pool-gw',
    pool_name: '核心云网关保留池 (Hubs)',
    cidr: '10.100.0.0/24',
    type: 'Gateway',
    allocation_mode: 'static',
    gateway_binding: '全球骨干网关集群',
    used: 4,
    total: 254,
    color: '#3b82f6',
    description: '全球云端骨干网关及 HA 虚拟漂移 IP 专用网段',
    status: 'Active',
  },
  {
    id: 'pool-dev',
    pool_name: '基础架构与研发部虚拟池 (Spokes)',
    cidr: '10.100.10.0/20',
    type: 'User-Device',
    allocation_mode: 'dynamic',
    idp_group: '研发核心组',
    used: 128,
    total: 4094,
    color: '#10b981',
    description: '通过飞书/Okta完成首次设备注册时动态下发绑定，永久锁定',
    status: 'Active',
  },
  {
    id: 'pool-sec',
    pool_name: '安全运维与特权访问专网池 (Spokes)',
    cidr: '10.100.30.0/20',
    type: 'User-Device',
    allocation_mode: 'dynamic',
    idp_group: '安全运维组',
    used: 32,
    total: 4094,
    color: '#8b5cf6',
    description: '信息安全、合规审计与 SRE 运维终端隔离网段',
    status: 'Active',
  },
  {
    id: 'pool-qa',
    pool_name: '测试集群与自动化沙箱设备池',
    cidr: '10.100.50.0/20',
    type: 'User-Device',
    allocation_mode: 'dynamic',
    idp_group: '产品测试组',
    used: 48,
    total: 4094,
    color: '#f59e0b',
    description: 'CI/CD 构建 Runner 节点与临时自动化测试设备动态租约',
    status: 'Active',
  },
  {
    id: 'pool-vip',
    pool_name: '高可用核心微服务浮动池 (Service-VIP)',
    cidr: '10.100.250.0/24',
    type: 'Service-VIP',
    allocation_mode: 'static',
    used: 6,
    total: 254,
    color: '#06b6d4',
    description: '网关隧道内 CoreDNS、认证中台等全局漂移服务 IP',
    status: 'Active',
  },
];

// Initial Mock IPAM Live Leases
export const initialIpamLeases: IpamLease[] = [
  {
    id: 'lease-gw-1',
    virtual_ip: '10.100.0.1',
    node_type: 'Gateway',
    target_name: 'gw-sh-branch-01 (阿里云华东网关)',
    endpoint: '47.92.12.3:51820',
    allocation_mode: '自动化分配-固定',
    pool_id: 'pool-gw',
    pool_name: '核心云网关保留池 (Hubs)',
    status: 'Active',
    wireguard_pubkey: 'bK0vR1lM9wN8oX3yZ2aQ5rS4tU7vW6xY9zA1bC2dE3f=',
    last_handshake: new Date(Date.now() - 1000 * 25).toISOString(),
    allocated_at: '2024-01-10T08:00:00Z',
  },
  {
    id: 'lease-gw-2',
    virtual_ip: '10.100.0.2',
    node_type: 'Gateway',
    target_name: 'gw-bj-hub-01 (北京总部核心网关)',
    endpoint: '202.108.22.5:51820',
    allocation_mode: '自动化分配-固定',
    pool_id: 'pool-gw',
    pool_name: '核心云网关保留池 (Hubs)',
    status: 'Active',
    wireguard_pubkey: 'aJ9uQ0kL8vM7nW2xY1zP4qR3sT6uV5wX8yZ0aB1cD2e=',
    last_handshake: new Date(Date.now() - 1000 * 12).toISOString(),
    allocated_at: '2024-01-12T09:30:00Z',
  },
  {
    id: 'lease-gw-3',
    virtual_ip: '10.100.0.3',
    node_type: 'Gateway',
    target_name: 'gw-sz-branch-01 (深圳创新中心网关)',
    endpoint: '119.29.35.80:51820',
    allocation_mode: '自动化分配-固定',
    pool_id: 'pool-gw',
    pool_name: '核心云网关保留池 (Hubs)',
    status: 'Active',
    wireguard_pubkey: 'cL1wS2mN0xO9pY4zA3bR6sT5uV8wX7yZ0aB2cD3eF4g=',
    last_handshake: new Date(Date.now() - 1000 * 45).toISOString(),
    allocated_at: '2024-01-15T10:15:00Z',
  },
  {
    id: 'lease-gw-4',
    virtual_ip: '10.100.0.4',
    node_type: 'Gateway',
    target_name: 'gw-fra-cloud-01 (法兰克福灾备网关)',
    endpoint: '185.12.64.92:51820',
    allocation_mode: '自动化分配-固定',
    pool_id: 'pool-gw',
    pool_name: '核心云网关保留池 (Hubs)',
    status: 'Active',
    wireguard_pubkey: 'dM2xT3nO1yP0qZ5aB4cS7tU6vW9xY8zA1bC3dE4fG5h=',
    last_handshake: new Date(Date.now() - 1000 * 180).toISOString(),
    allocated_at: '2024-02-01T14:20:00Z',
  },
  {
    id: 'lease-dev-1',
    virtual_ip: '10.100.10.1',
    node_type: 'Peer',
    target_name: '李娜-ThinkPad-T14 (dev-lead)',
    endpoint: '180.168.41.22:60114',
    allocation_mode: '飞书触发-动态锁定',
    pool_id: 'pool-dev',
    pool_name: '基础架构与研发部虚拟池 (Spokes)',
    status: 'Active',
    wireguard_pubkey: 'fO4zV5pQ3aR2sb7cD6eU9vW8xY1zA0bC3dE5fG6hI7j=',
    last_handshake: new Date(Date.now() - 1000 * 60).toISOString(),
    allocated_at: '2024-03-05T09:00:00Z',
  },
  {
    id: 'lease-dev-5',
    virtual_ip: '10.100.10.5',
    node_type: 'Peer',
    target_name: '张伟-MacBook-M3Max',
    endpoint: '124.65.178.6:58921',
    allocation_mode: '飞书触发-动态锁定',
    pool_id: 'pool-dev',
    pool_name: '基础架构与研发部虚拟池 (Spokes)',
    status: 'Active',
    wireguard_pubkey: 'gP5aW6qR4bS3tc8dE7fV0wX9yZ2aB1cD4eF6gH7iJ8k=',
    last_handshake: new Date(Date.now() - 1000 * 85).toISOString(),
    allocated_at: '2024-03-08T11:20:00Z',
  },
  {
    id: 'lease-dev-12',
    virtual_ip: '10.100.10.12',
    node_type: 'Server',
    target_name: 'ci-runner-ubuntu-01 (自动编译机)',
    endpoint: '202.108.22.18:54120',
    allocation_mode: '自动化分配-固定',
    pool_id: 'pool-dev',
    pool_name: '基础架构与研发部虚拟池 (Spokes)',
    status: 'Active',
    wireguard_pubkey: 'hQ6bX7rS5cT4ud9eF8gW1xY0zA3bC2dE5fG7hI8jK9l=',
    last_handshake: new Date(Date.now() - 1000 * 30).toISOString(),
    allocated_at: '2024-03-15T15:40:00Z',
  },
  {
    id: 'lease-sec-2',
    virtual_ip: '10.100.30.2',
    node_type: 'Peer',
    target_name: '王磊-iPhone15-Pro',
    endpoint: '124.65.178.99:62100',
    allocation_mode: '钉钉触发-动态锁定',
    pool_id: 'pool-sec',
    pool_name: '安全运维与特权访问专网池 (Spokes)',
    status: 'Active',
    wireguard_pubkey: 'iR7cY8sT6dU5ve0fG9hX2yZ1aB4cD3eF6gH8iJ9kL0m=',
    last_handshake: new Date(Date.now() - 1000 * 320).toISOString(),
    allocated_at: '2024-04-01T08:30:00Z',
  },
  {
    id: 'lease-sec-8',
    virtual_ip: '10.100.30.8',
    node_type: 'Peer',
    target_name: '刘洋-Dell-Precision (SRE特权机)',
    endpoint: '119.29.35.95:52310',
    allocation_mode: '飞书触发-动态锁定',
    pool_id: 'pool-sec',
    pool_name: '安全运维与特权访问专网池 (Spokes)',
    status: 'Active',
    wireguard_pubkey: 'jS8dZ9tU7eV6wf1gH0iY3zA2bC5dE4fG7hI9jK0lM1n=',
    last_handshake: new Date(Date.now() - 1000 * 120).toISOString(),
    allocated_at: '2024-04-10T12:00:00Z',
  },
  {
    id: 'lease-qa-15',
    virtual_ip: '10.100.50.15',
    node_type: 'Peer',
    target_name: '陈敏-Galaxy-S24 (移动端测试机)',
    endpoint: '139.196.88.42:57800',
    allocation_mode: '自动化分配-固定',
    pool_id: 'pool-qa',
    pool_name: '测试集群与自动化沙箱设备池',
    status: 'Idle',
    wireguard_pubkey: 'kT9ea0uV8fW7xg2hI1jZ4aB3cD6eF5gH8iJ0kL1mN2o=',
    last_handshake: new Date(Date.now() - 1000 * 3600 * 26).toISOString(),
    allocated_at: '2024-04-20T10:00:00Z',
  },
  {
    id: 'lease-vip-1',
    virtual_ip: '10.100.250.1',
    node_type: 'Server',
    target_name: 'CoreDNS-Cluster-VIP (隧道内首选解析)',
    endpoint: '内部隧道广播',
    allocation_mode: '管理员手动保留',
    pool_id: 'pool-vip',
    pool_name: '高可用核心微服务浮动池 (Service-VIP)',
    status: 'Reserved',
    wireguard_pubkey: 'VIP_STATIC_RESERVED_CORE_DNS',
    last_handshake: new Date().toISOString(),
    allocated_at: '2024-01-01T00:00:00Z',
  },
  {
    id: 'lease-vip-2',
    virtual_ip: '10.100.250.2',
    node_type: 'Server',
    target_name: 'MySQL-Master-VIP (生产库漂移IP)',
    endpoint: '内部隧道广播',
    allocation_mode: '管理员手动保留',
    pool_id: 'pool-vip',
    pool_name: '高可用核心微服务浮动池 (Service-VIP)',
    status: 'Reserved',
    wireguard_pubkey: 'VIP_STATIC_RESERVED_MYSQL_HA',
    last_handshake: new Date().toISOString(),
    allocated_at: '2024-01-01T00:00:00Z',
  },
];

// Generate Mock Audit Logs
export function generateMockAudit(): AuditRecord[] {
  const users = ['张伟', '李娜', '王磊', '刘洋', '陈敏', '系统自动化'];
  const devices = [
    '张伟-MacBook-M3Max',
    '李娜-ThinkPad-T14',
    '王磊-iPhone15-Pro',
    '刘洋-Dell-Precision',
    'ci-runner-ubuntu-01',
    'sec-scanner-worker',
  ];
  const nodes = ['gw-bj-hub-01', 'gw-sh-branch-01', 'gw-sz-branch-01', 'gw-fra-cloud-01'];
  const ips = [
    '10.0.1.101',
    '10.0.1.102',
    '10.0.1.103',
    '10.0.1.105',
    '10.0.0.1',
    '10.0.0.2',
    '192.168.10.50',
    '192.168.20.14',
    '192.168.30.99',
    '10.100.0.12',
  ];
  const ports = [22, 80, 443, 3306, 5432, 6379, 8080, 51820, 23, 9200];

  const records: AuditRecord[] = [];
  const baseTime = Date.now();

  for (let i = 0; i < 60; i++) {
    const isDeny = i % 7 === 0 || i % 13 === 0;
    const userIndex = i % users.length;
    const deviceIndex = i % devices.length;
    const nodeIndex = i % nodes.length;
    const srcIndex = i % 4;
    const dstIndex = 4 + (i % (ips.length - 4));
    const port = isDeny ? (i % 2 === 0 ? 23 : 8080) : ports[i % ports.length];
    const bytes = isDeny ? 0 : Math.floor(Math.random() * 850000) + 1200;

    records.push({
      id: `audit-${1000 + i}`,
      timestamp: new Date(baseTime - i * 1000 * 60 * (2 + (i % 5))).toISOString(),
      user: isDeny && i % 2 === 0 ? '未知/匿名探测' : users[userIndex],
      device: devices[deviceIndex],
      node: nodes[nodeIndex],
      sourceIp: ips[srcIndex],
      destIp: ips[dstIndex],
      port: port,
      action: isDeny ? 'deny' : 'allow',
      bytes: bytes,
    });
  }

  return records;
}

// Mock Peers generator for a node
export function getMockPeers(nodeId: string): Peer[] {
  const currentNodes = initialNodes.filter((n) => n.id !== nodeId);
  return currentNodes.map((n, idx) => ({
    id: `peer-${nodeId}-${n.id}`,
    nodeId: nodeId,
    peerNodeId: n.id,
    peerName: n.name,
    virtualIp: n.virtualIp,
    status: n.status,
    lastHandshake: n.lastHandshake,
    rxBytes: Math.floor(n.rxBytes / (idx + 2)),
    txBytes: Math.floor(n.txBytes / (idx + 2)),
    endpoint: n.endpoint,
    latencyMs: n.status === 'online' ? 8 + idx * 7 : n.status === 'degraded' ? 180 + idx * 30 : 0,
  }));
}

// Mock ACL generator for a node
export function getMockAcl(nodeId: string): AclRule[] {
  return [
    {
      id: `acl-${nodeId}-1`,
      direction: 'ingress',
      source: '10.0.0.0/16',
      destination: '10.0.1.0/24',
      port: '80, 443',
      action: 'allow',
      priority: 100,
      description: '允许全网 HTTP/HTTPS 业务流量进入',
    },
    {
      id: `acl-${nodeId}-2`,
      direction: 'ingress',
      source: '10.0.1.101/32, 10.0.1.102/32',
      destination: '10.0.0.0/24',
      port: '22',
      action: 'allow',
      priority: 200,
      description: '管理员及SRE终端接入 SSH 远程管理',
    },
    {
      id: `acl-${nodeId}-3`,
      direction: 'egress',
      source: 'any',
      destination: '0.0.0.0/0',
      port: '23, 21',
      action: 'deny',
      priority: 10,
      description: '禁止 Telnet / FTP 纯文本外发协议',
    },
    {
      id: `acl-${nodeId}-4`,
      direction: 'ingress',
      source: '10.0.1.0/24',
      destination: '192.168.10.50/32',
      port: '3306, 5432',
      action: 'allow',
      priority: 300,
      description: '安全组白名单数据库读写通道',
    },
    {
      id: `acl-${nodeId}-5`,
      direction: 'egress',
      source: 'any',
      destination: '10.0.0.0/8',
      port: '*',
      action: 'allow',
      priority: 999,
      description: 'Mesh 内部默认互联规则',
    },
  ];
}

export const initialEnrollmentTokens: EnrollmentToken[] = [
  {
    id: 'tok-wg-sh-prod-01',
    token: 'vt_tok_7a9f8e2c0d4b6a1e3f5b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f',
    nodeId: 'node-sh-prod-gw-01',
    nodeName: 'shanghai-prod-gateway-01',
    nodeType: '站点接入网关',
    networkName: '上海生产网',
    allowedSubnets: ['10.20.0.0/16'],
    nodeRoles: ['流量转发', '站点互联'],
    registrationMethod: 'token',
    requireApproval: true,
    approvalStatus: 'approved',
    approvedBy: 'sec_officer (安全保密员)',
    approvedAt: new Date(Date.now() - 1000 * 3600 * 2).toISOString(),
    permissions: ['node:register_only'],
    maxUsage: 1,
    usedCount: 0,
    status: 'PENDING',
    createdAt: new Date(Date.now() - 1000 * 3600 * 2).toISOString(),
    createdBy: 'admin (系统管理员)',
    expiresAt: new Date(Date.now() + 1000 * 3600 * 22).toISOString(),
    installCommand: 'curl -fsSL https://get.velotrust.internal/gateway-agent.sh | bash -s -- --token vt_tok_7a9f8e2c0d4b6a1e3f5b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f --server https://mesh-control.velotrust.internal --node shanghai-prod-gateway-01',
    configSnippet: `[Interface]\nPrivateKey = <AUTO_GENERATED_ON_NODE_BOOT>\nAddress = 10.0.0.10/24\nDNS = 10.100.250.1\n\n[Peer]\nPublicKey = aJ9uQ0kL8vM7nW2xY1zP4qR3sT6uV5wX8yZ0aB1cD2e=\nEndpoint = 202.108.22.5:51820\nAllowedIPs = 10.0.0.0/16, 10.20.0.0/16\nPersistentKeepalive = 25`,
    notes: '上海生产网第一接入网关，仅用于站点互联及子网 10.20.0.0/16 流量路由',
  },
  {
    id: 'tok-wg-bj-core-02',
    token: 'vt_tok_1b3c5e7a9d0f2b4e6a8c0e2d4f6a8b0c2d4e6f8a0b2c4d6e8f0a2b4c6d8e0f2a',
    nodeId: 'node-hub-bj',
    nodeName: 'gw-bj-hub-01',
    nodeType: '站点接入网关',
    networkName: '北京核心生产网',
    allowedSubnets: ['10.0.0.0/16', '10.100.0.0/16'],
    nodeRoles: ['流量转发', '站点互联'],
    registrationMethod: 'token',
    requireApproval: false,
    permissions: ['node:register_only'],
    maxUsage: 1,
    usedCount: 1,
    status: 'USED',
    createdAt: new Date(Date.now() - 1000 * 3600 * 72).toISOString(),
    createdBy: 'admin',
    expiresAt: new Date(Date.now() - 1000 * 3600 * 48).toISOString(),
    usedAt: new Date(Date.now() - 1000 * 3600 * 71).toISOString(),
    usedIp: '202.108.22.5',
    deviceFingerprint: 'Linux 5.15.0-x86_64-Ubuntu-22.04-LTS (UUID: 4e91c7a0-0d3f)',
    installCommand: 'curl -fsSL https://get.velotrust.internal/gateway-agent.sh | bash -s -- --token vt_tok_1b3c5e7a... --server https://mesh-control.velotrust.internal',
    configSnippet: `[Interface]\nAddress = 10.0.0.1/24\n...`,
    notes: '北京核心网关已完成入网握手，凭证已按照安全策略立即销毁失效',
  },
  {
    id: 'tok-wg-gz-edge-03',
    token: 'vt_tok_3c5e7a9b1d3f5a7b9c1d3e5f7a9b1c3d5e7f9a1b3c5d7e9f1a3b5c7d9e1f3a5b',
    nodeId: 'node-gw-fra',
    nodeName: 'gw-fra-cloud-01',
    nodeType: '跨国灾备网关',
    networkName: '法兰克福灾备专网',
    allowedSubnets: ['10.30.0.0/16'],
    nodeRoles: ['站点互联'],
    registrationMethod: 'token',
    requireApproval: true,
    permissions: ['node:register_only'],
    maxUsage: 1,
    usedCount: 0,
    status: 'EXPIRED',
    createdAt: new Date(Date.now() - 1000 * 3600 * 96).toISOString(),
    createdBy: 'net_architect',
    expiresAt: new Date(Date.now() - 1000 * 3600 * 24).toISOString(),
    installCommand: 'curl -fsSL https://get.velotrust.internal/gateway-agent.sh | bash -s -- --token vt_tok_3c5e7a9b... --server https://mesh-control.velotrust.internal',
    configSnippet: `[Interface]\nAddress = 10.0.0.4/24\n...`,
    notes: '未在有效时间内完成现场机器初始化，凭证超时自动失效',
  },
  {
    id: 'tok-wg-cd-dev-04',
    token: 'vt_tok_5e7a9b1c3d5f7a9b1c3d5e7f9a1b3c5d7e9f1a3b5c7d9e1f3a5b7c9d1e3f5a7b',
    nodeId: 'node-gw-sz',
    nodeName: 'gw-sz-branch-01',
    nodeType: '服务代理网关',
    networkName: '深圳创新基地内网',
    allowedSubnets: ['10.50.0.0/16'],
    nodeRoles: ['流量转发'],
    registrationMethod: 'token',
    requireApproval: true,
    permissions: ['node:register_only'],
    maxUsage: 1,
    usedCount: 0,
    status: 'REVOKED',
    createdAt: new Date(Date.now() - 1000 * 3600 * 48).toISOString(),
    createdBy: 'admin',
    expiresAt: new Date(Date.now() + 1000 * 3600 * 24).toISOString(),
    revokedAt: new Date(Date.now() - 1000 * 3600 * 12).toISOString(),
    revokedBy: 'sec_officer (安全保密员)',
    revokeReason: '网关硬件采购方案变更，作废原先生成的预注册凭证',
    installCommand: 'curl -fsSL https://get.velotrust.internal/gateway-agent.sh | bash -s -- --token vt_tok_5e7a9b1c... --server https://mesh-control.velotrust.internal',
    configSnippet: `[Interface]\nAddress = 10.0.0.3/24\n...`,
    notes: '安全保密员主动撤回作废',
  },
];

// In-Memory Data Store for interactive simulation
let nodesStore = [...initialNodes];
let usersStore = [...initialUsers];
let groupsStore = [...initialGroups];
let devicesStore = [...initialDevices];
let sitesStore = [...initialSites];
let policiesStore = [...initialPolicies];
let dnsStore = [...initialDnsZones];
let idpStore = [...initialIdpProviders];
let ipamPoolsStore = [...initialIpamPools];
let ipamLeasesStore = [...initialIpamLeases];
let auditStore = generateMockAudit();
let rolesStore = [...initialRoles];
let protectedResourcesStore = [...initialProtectedResources];
let menusStore = [...initialSysMenus];
let authResourcesStore = [...initialAuthResources];
let permissionsStore = [...initialSysPermissions];
let apiResourcesStore = [...initialApiResources];
let apiRpcMappingsStore = [...initialApiRpcMappings];
let loginLogsStore = [...initialLoginLogs];
let operationLogsStore = [...initialOperationLogs];
let enrollmentTokensStore = [...initialEnrollmentTokens];

let failSafeConfigStore: FailSafeConfig = {
  heartbeatIntervalSec: 15,
  offlineThresholdSec: 60,
  mode: 'ALLOW_EXISTING_CACHE', // 默认推荐：继续沿用本地缓存，并阻断新未授权设备
  localCacheTtlHours: 72,
  deadMansSwitchEnabled: true,
  deadMansTimeoutMinutes: 30,
  emergencyAdminBypass: true,
  lastDrillRun: '2024-03-01 10:00:00 (自动演练通过)',
  autoRecovery: true,
  lastSyncAt: new Date().toISOString(),
  gatewaysInFailSafe: 0,
};

const delay = (ms = 250) => new Promise((resolve) => setTimeout(resolve, ms));

export const mockApi = {
  // Auth
  async login(username: string, _password?: string) {
    await delay(300);
    const existing = usersStore.find((u) => u.username === username);
    return {
      token: 'jwt-mock-wireguard-' + Math.random().toString(36).substring(2),
      user: {
        username: username || 'admin',
        name: existing ? existing.name : '管理员 (' + (username || 'admin') + ')',
        role: existing && existing.roles.includes('admin') ? 'admin' : 'user',
        email: existing ? existing.email : `${username || 'admin'}@mesh.internal`,
      },
    };
  },

  // Nodes
  async getNodes() {
    await delay();
    return [...nodesStore];
  },

  async getNode(id: string) {
    await delay();
    const node = nodesStore.find((n) => n.id === id);
    if (!node) throw new Error('节点不存在');
    return { ...node };
  },

  async updateNode(id: string, updateData: Partial<Node>) {
    await delay();
    nodesStore = nodesStore.map((n) => (n.id === id ? { ...n, ...updateData } : n));
    const updated = nodesStore.find((n) => n.id === id);
    if (!updated) throw new Error('节点不存在');
    return { ...updated };
  },

  async deleteNode(id: string) {
    await delay();
    nodesStore = nodesStore.filter((n) => n.id !== id);
    // Add audit log
    auditStore.unshift({
      id: `audit-${Date.now()}`,
      timestamp: new Date().toISOString(),
      user: 'admin',
      device: 'Web 控制台',
      node: 'gw-bj-hub-01',
      sourceIp: '10.0.1.101',
      destIp: '10.0.0.1',
      port: 51820,
      action: 'allow',
      bytes: 256,
    });
    return true;
  },

  async getPeers(nodeId: string) {
    await delay();
    return getMockPeers(nodeId);
  },

  async getAcl(nodeId: string) {
    await delay();
    return getMockAcl(nodeId);
  },

  // Users
  async getUsers() {
    await delay();
    return [...usersStore];
  },

  async createUser(userData: Omit<User, 'id' | 'createdAt' | 'lastLogin'>) {
    await delay();
    const newUser: User = {
      ...userData,
      id: `user-${Date.now()}`,
      createdAt: new Date().toISOString(),
      lastLogin: new Date().toISOString(),
    };
    usersStore.unshift(newUser);

    // Update group member count if groups assigned
    if (userData.groups && userData.groups.length > 0) {
      groupsStore = groupsStore.map((g) => {
        if (userData.groups.includes(g.name)) {
          return {
            ...g,
            memberCount: g.memberCount + 1,
            memberIds: [...g.memberIds, newUser.id],
          };
        }
        return g;
      });
    }

    return newUser;
  },

  async updateUser(id: string, userData: Partial<User>) {
    await delay();
    usersStore = usersStore.map((u) => (u.id === id ? { ...u, ...userData } : u));
    return usersStore.find((u) => u.id === id)!;
  },

  async toggleUserStatus(id: string) {
    await delay();
    usersStore = usersStore.map((u) =>
      u.id === id ? { ...u, status: u.status === 'active' ? 'disabled' : 'active' } : u
    );
    return usersStore.find((u) => u.id === id)!;
  },

  async deleteUser(id: string) {
    await delay();
    usersStore = usersStore.filter((u) => u.id !== id);
    groupsStore = groupsStore.map((g) => ({
      ...g,
      memberIds: g.memberIds.filter((mid) => mid !== id),
      memberCount: g.memberIds.filter((mid) => mid !== id).length,
    }));
    return true;
  },

  // Groups
  async getGroups() {
    await delay();
    return [...groupsStore];
  },

  async createGroup(data: { name: string; description: string }) {
    await delay();
    const newGroup: Group = {
      id: `group-${Date.now()}`,
      name: data.name,
      description: data.description,
      memberCount: 0,
      memberIds: [],
    };
    groupsStore.unshift(newGroup);
    return newGroup;
  },

  async updateGroup(id: string, data: { name: string; description: string }) {
    await delay();
    groupsStore = groupsStore.map((g) => (g.id === id ? { ...g, ...data } : g));
    return groupsStore.find((g) => g.id === id)!;
  },

  async updateGroupMembers(groupId: string, memberIds: string[]) {
    await delay();
    const group = groupsStore.find((g) => g.id === groupId);
    if (!group) throw new Error('用户组不存在');

    groupsStore = groupsStore.map((g) =>
      g.id === groupId
        ? {
            ...g,
            memberIds,
            memberCount: memberIds.length,
          }
        : g
    );

    // Synchronize users' groups array
    usersStore = usersStore.map((u) => {
      const isMember = memberIds.includes(u.id);
      const currentGroups = u.groups || [];
      if (isMember && !currentGroups.includes(group.name)) {
        return { ...u, groups: [...currentGroups, group.name] };
      }
      if (!isMember && currentGroups.includes(group.name)) {
        return { ...u, groups: currentGroups.filter((gn) => gn !== group.name) };
      }
      return u;
    });

    return groupsStore.find((g) => g.id === groupId)!;
  },

  async deleteGroup(id: string) {
    await delay();
    const group = groupsStore.find((g) => g.id === id);
    if (group) {
      usersStore = usersStore.map((u) => ({
        ...u,
        groups: u.groups.filter((gn) => gn !== group.name),
      }));
    }
    groupsStore = groupsStore.filter((g) => g.id !== id);
    return true;
  },

  // Devices
  async getDevices() {
    await delay();
    return [...devicesStore];
  },

  async registerDevice(data: {
    userId: string;
    siteId: string;
    type: Device['type'];
    deviceName?: string;
  }) {
    await delay();
    const user = usersStore.find((u) => u.id === data.userId);
    const site = sitesStore.find((s) => s.id === data.siteId);
    const deviceCount = devicesStore.length + 1;
    const ipLast = 100 + deviceCount;

    const token = `wg-tok-${Math.random().toString(36).substring(2, 10)}-${Date.now().toString(36)}`;
    const newDevice: Device = {
      id: `dev-${Date.now()}`,
      name:
        data.deviceName ||
        `${user ? user.name.split(' ')[0] : '用户'}-${data.type}-${Math.random().toString(36).substring(2, 6).toUpperCase()}`,
      userId: data.userId,
      userName: user ? user.name : '未知用户',
      type: data.type,
      os:
        data.type === 'PC'
          ? 'macOS / Windows / Linux'
          : data.type === 'Phone'
            ? 'iOS / Android'
            : 'iPadOS',
      virtualIp: `10.0.1.${ipLast}`,
      status: 'active',
      lastSeen: new Date().toISOString(),
      siteId: data.siteId,
      siteName: site ? site.name : '北京总部核心区',
      token: token,
      createdAt: new Date().toISOString(),
    };

    devicesStore.unshift(newDevice);
    return newDevice;
  },

  async revokeDevice(id: string) {
    await delay();
    devicesStore = devicesStore.map((d) => (d.id === id ? { ...d, status: 'revoked' } : d));
    return devicesStore.find((d) => d.id === id)!;
  },

  async deleteDevice(id: string) {
    await delay();
    devicesStore = devicesStore.filter((d) => d.id !== id);
    return true;
  },

  // Sites
  async getSites() {
    await delay();
    return [...sitesStore];
  },

  async createSite(data: { name: string; localSubnet: string; gatewayNodeId: string }) {
    await delay();
    const gwNode = nodesStore.find((n) => n.id === data.gatewayNodeId);
    const newSite: Site = {
      id: `site-${Date.now()}`,
      name: data.name,
      localSubnet: data.localSubnet,
      gatewayNodeId: data.gatewayNodeId,
      gatewayNodeName: gwNode ? gwNode.name : '未指定网关',
      nodeCount: 1,
      status: 'active',
      createdAt: new Date().toISOString(),
    };
    sitesStore.unshift(newSite);
    return newSite;
  },

  async updateSite(
    id: string,
    data: { name: string; localSubnet: string; gatewayNodeId: string; status?: 'active' | 'inactive' }
  ) {
    await delay();
    const gwNode = nodesStore.find((n) => n.id === data.gatewayNodeId);
    sitesStore = sitesStore.map((s) =>
      s.id === id
        ? {
            ...s,
            ...data,
            gatewayNodeName: gwNode ? gwNode.name : s.gatewayNodeName,
          }
        : s
    );
    return sitesStore.find((s) => s.id === id)!;
  },

  async deleteSite(id: string) {
    await delay();
    sitesStore = sitesStore.filter((s) => s.id !== id);
    return true;
  },

  // Policies
  async getPolicies() {
    await delay();
    return [...policiesStore];
  },

  async createPolicy(data: Omit<Policy, 'id' | 'createdAt'>) {
    await delay();
    const newPolicy: Policy = {
      ...data,
      id: `pol-${Date.now()}`,
      createdAt: new Date().toISOString(),
    };
    policiesStore.unshift(newPolicy);
    return newPolicy;
  },

  async updatePolicy(id: string, data: Partial<Policy>) {
    await delay();
    policiesStore = policiesStore.map((p) => (p.id === id ? { ...p, ...data } : p));
    return policiesStore.find((p) => p.id === id)!;
  },

  async togglePolicy(id: string) {
    await delay();
    policiesStore = policiesStore.map((p) => (p.id === id ? { ...p, enabled: !p.enabled } : p));
    return policiesStore.find((p) => p.id === id)!;
  },

  async deletePolicy(id: string) {
    await delay();
    policiesStore = policiesStore.filter((p) => p.id !== id);
    return true;
  },

  async compileNodeAcl(nodeId: string) {
    await delay(350);
    const node = nodesStore.find((n) => n.id === nodeId);
    const rules = getMockAcl(nodeId);
    return {
      nodeId,
      nodeName: node ? node.name : 'Unknown Node',
      version: `v2.4.18-rev-${Math.random().toString(16).substring(2, 8)}`,
      compiledAt: new Date().toISOString(),
      checksum: `sha256:${Math.random().toString(36).substring(2, 14)}` + `${Math.random().toString(36).substring(2, 14)}`,
      rules: rules,
    };
  },

  // Audit
  async getAudit() {
    await delay();
    return [...auditStore];
  },

  // Dashboard Stats
  async getDashboardStats(): Promise<DashboardStats> {
    await delay();
    const onlineNodes = nodesStore.filter((n) => n.status === 'online').length;
    const offlineNodes = nodesStore.filter((n) => n.status === 'offline').length;
    const degradedNodes = nodesStore.filter((n) => n.status === 'degraded').length;

    // Traffic trend simulation for last 24 hours (every 2 hours)
    const trafficTrend = [
      { time: '00:00', rx: 340, tx: 280 },
      { time: '02:00', rx: 180, tx: 120 },
      { time: '04:00', rx: 120, tx: 90 },
      { time: '06:00', rx: 210, tx: 160 },
      { time: '08:00', rx: 580, tx: 490 },
      { time: '10:00', rx: 890, tx: 740 },
      { time: '12:00', rx: 720, tx: 610 },
      { time: '14:00', rx: 940, tx: 820 },
      { time: '16:00', rx: 1020, tx: 880 },
      { time: '18:00', rx: 810, tx: 690 },
      { time: '20:00', rx: 620, tx: 510 },
      { time: '22:00', rx: 450, tx: 380 },
    ];

    return {
      totalNodes: nodesStore.length,
      onlineNodes,
      offlineNodes,
      degradedNodes,
      totalUsers: usersStore.length,
      totalDevices: devicesStore.length,
      totalSites: sitesStore.length,
      statusDistribution: [
        { label: '在线 (Online)', value: onlineNodes, color: '#10b981' },
        { label: '降级 (Degraded)', value: degradedNodes, color: '#f59e0b' },
        { label: '离线 (Offline)', value: offlineNodes, color: '#94a3b8' },
      ],
      trafficTrend,
      recentAudit: auditStore.slice(0, 10),
    };
  },

  // --- 智能内网 DNS (Split DNS) ---
  async getDnsZones() {
    await delay();
    return [...dnsStore];
  },

  async getDnsZone(id: string) {
    await delay();
    const zone = dnsStore.find((z) => z.id === id || z.zone_id === id);
    if (!zone) throw new Error('DNS 区域未找到');
    return { ...zone };
  },

  async createDnsZone(data: Omit<DnsZone, 'id' | 'createdAt' | 'peer_count'>) {
    await delay();
    const newZone: DnsZone = {
      ...data,
      id: `dns-${Date.now()}`,
      peer_count: 12 + Math.floor(Math.random() * 30),
      createdAt: new Date().toISOString(),
    };
    dnsStore.unshift(newZone);
    return newZone;
  },

  async updateDnsZone(id: string, data: Partial<DnsZone>) {
    await delay();
    dnsStore = dnsStore.map((z) => (z.id === id ? { ...z, ...data } : z));
    return dnsStore.find((z) => z.id === id)!;
  },

  async toggleDnsZone(id: string) {
    await delay();
    dnsStore = dnsStore.map((z) =>
      z.id === id ? { ...z, status: z.status === 'Active' ? 'Paused' : 'Active' } : z
    );
    return dnsStore.find((z) => z.id === id)!;
  },

  async deleteDnsZone(id: string) {
    await delay();
    dnsStore = dnsStore.filter((z) => z.id !== id);
    return true;
  },

  // DNS Resolution Simulator: simulates client-side split DNS resolution
  async simulateDnsQuery(queryDomain: string, clientGroupName: string) {
    await delay(300);
    const domain = queryDomain.trim().toLowerCase();

    // Check if domain matches any active zone's suffix
    const matchedZone = dnsStore.find(
      (z) => z.status === 'Active' && (domain.endsWith(`.${z.domain_suffix}`) || domain === z.domain_suffix)
    );

    if (!matchedZone) {
      return {
        domain,
        isIntercepted: false,
        reason: '未命中任何已注册的内网 Split DNS 后缀规则，直连公网运营商 DNS (8.8.8.8 / 114.114.114.114) 解析',
        serverUsed: '公网本地 DNS (Public DNS)',
        resolvedIps: ['203.119.214.100 (模拟公网 IP)'],
        latencyMs: 14,
        clientHasAccess: true,
      };
    }

    // Check group access (DNS view isolation)
    const hasGroupAccess =
      clientGroupName === '*' ||
      matchedZone.assigned_groups.length === 0 ||
      matchedZone.assigned_groups.includes(clientGroupName);

    if (!hasGroupAccess) {
      return {
        domain,
        isIntercepted: true,
        zoneSuffix: matchedZone.domain_suffix,
        reason: `命中内网区域 *.${matchedZone.domain_suffix}，但当前用户所在安全组 [${clientGroupName}] 未被授权加载该 DNS 视界路由（防止企业内网拓扑嗅探泄露）`,
        serverUsed: '隧道网关拒绝 (NXDOMAIN)',
        resolvedIps: [],
        latencyMs: 3,
        clientHasAccess: false,
      };
    }

    // Find static record if any
    const hostPrefix = domain.replace(`.${matchedZone.domain_suffix}`, '');
    const matchedRecord = matchedZone.records.find((r) => r.host === hostPrefix);

    return {
      domain,
      isIntercepted: true,
      zoneSuffix: matchedZone.domain_suffix,
      reason: `命中智能 Split DNS 策略！由加密 WireGuard 隧道定向转发至网关上游解析器`,
      serverUsed: matchedZone.dns_servers[0] || '10.0.0.1',
      gateway: matchedZone.bind_gateway,
      recordType: matchedRecord?.type || 'A',
      resolvedIps: matchedRecord
        ? [matchedRecord.value]
        : [`10.${Math.floor(Math.random() * 200)}.${Math.floor(Math.random() * 200)}.15`],
      latencyMs: 5,
      clientHasAccess: true,
    };
  },

  // --- 第三方身份提供商 (IdP Integration) ---
  async getIdpConfigs() {
    await delay();
    return [...idpStore];
  },

  async getIdpConfig(id: string) {
    await delay();
    const config = idpStore.find((i) => i.id === id || i.provider_id === id);
    if (!config) throw new Error('认证源配置未找到');
    return { ...config };
  },

  async updateIdpConfig(id: string, data: Partial<IdpProvider>) {
    await delay();
    idpStore = idpStore.map((i) => (i.id === id ? { ...i, ...data } : i));
    return idpStore.find((i) => i.id === id)!;
  },

  async testIdpConnection(id: string) {
    await delay(500);
    const provider = idpStore.find((i) => i.id === id);
    if (!provider) throw new Error('认证源未找到');
    const isSuccess = provider.issuer_url.startsWith('https://');
    return {
      success: isSuccess,
      latencyMs: isSuccess ? 68 : 0,
      issuer: provider.issuer_url,
      message: isSuccess
        ? `成功探测到 OIDC 发现文档: ${provider.issuer_url}/.well-known/openid-configuration (状态 HTTP 200 OK)`
        : '无法连通 Issuer URL，请检查网络路由与端点配置',
      timestamp: new Date().toISOString(),
    };
  },

  async syncIdpUsers(id: string) {
    await delay(600);
    const now = new Date();
    const syncTimeStr = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')} ${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}:${String(now.getSeconds()).padStart(2, '0')}`;
    idpStore = idpStore.map((i) =>
      i.id === id
        ? {
            ...i,
            sync_time: syncTimeStr,
            status: 'Connected',
            user_count: i.user_count + Math.floor(Math.random() * 5),
          }
        : i
    );
    return idpStore.find((i) => i.id === id)!;
  },

  // JWT Token 解析试算器: 计算 IdP Token -> 安全组 -> 动态下发的路由与策略
  async simulateIdpToken(payload: {
    providerId: string;
    sub: string;
    email: string;
    idpGroups: string[];
  }) {
    await delay(400);
    const provider = idpStore.find((p) => p.id === payload.providerId || p.provider_id === payload.providerId);
    const mappings = provider ? provider.group_mapping : [];

    // Map IdP groups to VeloTrust groups
    const resolvedVeloGroups: string[] = [];
    payload.idpGroups.forEach((idpG) => {
      const match = mappings.find((m) => m.idp_group_name.trim() === idpG.trim());
      if (match) {
        resolvedVeloGroups.push(match.velotrust_group_id);
      }
    });

    // Deduplicate
    const finalGroups = Array.from(new Set(resolvedVeloGroups));

    // Find applicable network policies for these groups
    const matchedPolicies = policiesStore.filter(
      (p) =>
        p.enabled &&
        (finalGroups.some((g) => p.subject === `group:${g}`) || p.subject === '*')
    );

    // Find assigned DNS zones
    const matchedDns = dnsStore.filter(
      (z) =>
        z.status === 'Active' &&
        finalGroups.some((g) => z.assigned_groups.includes(g))
    );

    return {
      authenticatedUser: {
        sub: payload.sub,
        email: payload.email,
        provider: provider ? provider.name : 'Unknown IdP',
      },
      resolvedGroups: finalGroups,
      matchedPolicies: matchedPolicies.map((p) => ({
        name: p.name,
        targetIp: p.targetIp,
        ports: p.ports,
        action: p.action,
      })),
      accessibleDnsZones: matchedDns.map((z) => `*.${z.domain_suffix}`),
      sessionTtlHours: provider ? provider.session_ttl_hours : 12,
      expiryTimestamp: new Date(Date.now() + (provider ? provider.session_ttl_hours : 12) * 3600 * 1000).toISOString(),
    };
  },

  // --- 访问权限透视矩阵与可达性计算 (Access Matrix) ---
  async getAccessMatrix(subjectType: 'group' | 'user' | 'device', subjectId: string) {
    await delay(300);

    let subjectInfo: any = {
      type: subjectType,
      id: subjectId,
      name: '',
      subtitle: '',
      virtualIp: '',
      status: 'active',
      isBlocked: false,
      blockReason: '',
      groups: [] as string[],
    };

    if (subjectType === 'group') {
      const g = groupsStore.find((grp) => grp.id === subjectId || grp.name === subjectId);
      if (!g) throw new Error('用户组未找到');
      subjectInfo.id = g.id;
      subjectInfo.name = g.name;
      subjectInfo.subtitle = `${g.memberCount} 位成员 • ${g.description}`;
      subjectInfo.groups = [g.name];
    } else if (subjectType === 'user') {
      const u = usersStore.find((usr) => usr.id === subjectId || usr.username === subjectId);
      if (!u) throw new Error('用户未找到');
      subjectInfo.id = u.id;
      subjectInfo.name = `${u.name} (${u.username})`;
      subjectInfo.subtitle = `${u.department} • 角色: ${u.roles.join(', ')}`;
      subjectInfo.groups = u.groups || [];
      if (u.status === 'disabled') {
        subjectInfo.isBlocked = true;
        subjectInfo.blockReason = '该账号已被管理员在系统用户中心全局禁用，所有隧道访问已熔断';
      }
    } else if (subjectType === 'device') {
      const d = devicesStore.find((dev) => dev.id === subjectId);
      if (!d) throw new Error('设备未找到');
      const owner = usersStore.find((usr) => usr.id === d.userId);
      subjectInfo.id = d.id;
      subjectInfo.name = d.name;
      subjectInfo.subtitle = `员工: ${d.userName} • 操作系统: ${d.os} (${d.type})`;
      subjectInfo.virtualIp = d.virtualIp;
      subjectInfo.status = d.status;
      subjectInfo.groups = owner?.groups || [];
      if (d.status === 'revoked') {
        subjectInfo.isBlocked = true;
        subjectInfo.blockReason = '该设备入网证书已在控制面被强制撤销 (Revoked)，WireGuard 握手被直接切断';
      }
    }

    // If completely blocked
    if (subjectInfo.isBlocked) {
      return {
        subject: subjectInfo,
        stats: {
          accessibleSubnetsCount: 0,
          accessiblePortsCount: 0,
          accessibleDnsCount: 0,
          deniedRulesCount: policiesStore.length,
          securityScore: 0,
        },
        accessibleSubnets: [],
        keyEndpoints: [],
        dnsZones: [],
        deniedAssets: [
          {
            target: '全部内网资产 (0.0.0.0/0)',
            ports: '*',
            reason: subjectInfo.blockReason,
            policyName: '零信任安全阻断策略',
          },
        ],
        flowNodes: [
          { id: 'sub', label: subjectInfo.name, type: 'subject', status: 'deny' },
          { id: 'gw-block', label: '零信任阻断网关', type: 'gateway', status: 'deny' },
        ],
        flowEdges: [
          { source: 'sub', target: 'gw-block', action: 'deny', label: '权限已熔断' },
        ],
      };
    }

    // Match policies applicable to this subject's groups or username
    const applicablePolicies = policiesStore.filter((p) => {
      if (!p.enabled) return false;
      if (p.subject === '*') return true;
      if (subjectInfo.groups.some((g: string) => p.subject === `group:${g}`)) return true;
      if (subjectType === 'user' && p.subject === `user:${subjectInfo.name}`) return true;
      return false;
    });

    const allowPolicies = applicablePolicies.filter((p) => p.action === 'allow');
    const denyPolicies = applicablePolicies.filter((p) => p.action === 'deny');

    // Build accessible subnets
    const accessibleSubnets = sitesStore
      .filter((s) => s.status === 'active')
      .map((s) => {
        const matchingPolicy = allowPolicies.find(
          (p) =>
            p.targetIp.includes(s.localSubnet.slice(0, 9)) ||
            p.targetIp === '0.0.0.0/0' ||
            p.targetIp === '10.0.0.0/16' ||
            p.targetIp.includes('192.168.')
        );
        return {
          name: s.name,
          cidr: s.localSubnet,
          siteName: s.name,
          gatewayName: s.gatewayNodeName,
          policyName: matchingPolicy ? matchingPolicy.name : '默认互通策略',
          ports: matchingPolicy ? matchingPolicy.ports : '80,443',
          status: 'allow' as const,
        };
      });

    // Key services & DB endpoints
    const allServices = [
      {
        name: '生产 MySQL 主数据库',
        ip: '192.168.10.50',
        port: '3306',
        protocol: 'TCP / MySQL',
        category: 'database' as const,
        allowedGroup: '安全运维组',
      },
      {
        name: '生产 PostgreSQL 集群',
        ip: '192.168.10.50',
        port: '5432',
        protocol: 'TCP / PGSQL',
        category: 'database' as const,
        allowedGroup: '安全运维组',
      },
      {
        name: '上海预发测试集群 K8s',
        ip: '192.168.20.100',
        port: '8000-9000',
        protocol: 'HTTP/gRPC',
        category: 'cluster' as const,
        allowedGroup: '研发核心组',
      },
      {
        name: '核心 Linux 堡垒机',
        ip: '10.0.0.1',
        port: '22',
        protocol: 'SSH',
        category: 'management' as const,
        allowedGroup: '安全运维组',
      },
      {
        name: 'ELK / Kibana 日志平台',
        ip: '192.168.10.88',
        port: '5601',
        protocol: 'HTTP',
        category: 'service' as const,
        allowedGroup: '第三方审计组',
      },
      {
        name: '代码仓库 GitLab / CI',
        ip: '10.0.1.103',
        port: '80, 443',
        protocol: 'HTTP/HTTPS',
        category: 'service' as const,
        allowedGroup: '*',
      },
    ];

    const accessibleServices = allServices.filter((svc) => {
      if (svc.allowedGroup === '*') return true;
      return subjectInfo.groups.includes(svc.allowedGroup);
    });

    // Accessible DNS zones
    const accessibleDnsZones = dnsStore
      .filter((z) => z.status === 'Active')
      .filter(
        (z) =>
          z.assigned_groups.length === 0 ||
          subjectInfo.groups.some((g: string) => z.assigned_groups.includes(g))
      )
      .map((z) => ({
        suffix: `*.${z.domain_suffix}`,
        description: z.description,
        gateway: z.bind_gateway || '默认网关',
        recordsCount: z.records.length,
      }));

    // Explicit Denied Assets
    const deniedAssets = [
      {
        target: '非标明文管理端口 (0.0.0.0/0)',
        ports: '21, 23 (Telnet/FTP)',
        reason: '全网策略默认阻断明文高危协议传输',
        policyName: '全网禁用明文 Telnet 与非标管理端口',
      },
    ];

    if (!subjectInfo.groups.includes('安全运维组')) {
      deniedAssets.push({
        target: '生产核心数据库 (192.168.10.50)',
        ports: '3306, 5432, 6379',
        reason: '仅安全运维组白名单放行，未授权跨组接入',
        policyName: '生产数据库专网只允许运维组接入',
      });
    }

    if (!subjectInfo.groups.includes('第三方审计组') && !subjectInfo.groups.includes('安全运维组')) {
      deniedAssets.push({
        target: 'ELK / Kibana 敏感监控平台 (192.168.10.88)',
        ports: '5601, 9200',
        reason: '审计平台仅限合规审计人员接入',
        policyName: '审计人员只读访问监控指标与日志平台',
      });
    }

    // Build SVG Flow Nodes and Edges
    const flowNodes: any[] = [
      { id: 'subj', label: subjectInfo.name, type: 'subject', status: 'online' },
      { id: 'gw-bj', label: '北京中心网关 (gw-bj-hub-01)', type: 'gateway', status: 'online' },
      { id: 'gw-sh', label: '上海研发网关 (gw-sh-branch-01)', type: 'gateway', status: 'online' },
    ];

    accessibleSubnets.slice(0, 3).forEach((sub, i) => {
      flowNodes.push({
        id: `res-sub-${i}`,
        label: `${sub.name} (${sub.cidr})`,
        type: 'subnet',
        status: 'allow',
      });
    });

    accessibleServices.slice(0, 3).forEach((svc, i) => {
      flowNodes.push({
        id: `res-svc-${i}`,
        label: `${svc.name} (:${svc.port})`,
        type: 'service',
        status: 'allow',
      });
    });

    const flowEdges: any[] = [
      { source: 'subj', target: 'gw-bj', action: 'allow', label: 'WireGuard 隧道' },
      { source: 'subj', target: 'gw-sh', action: 'allow', label: 'WireGuard 隧道' },
      { source: 'gw-bj', target: 'res-sub-0', action: 'allow', label: 'AllowedIPs 路由' },
      { source: 'gw-sh', target: 'res-sub-1', action: 'allow', label: 'AllowedIPs 路由' },
    ];

    if (accessibleServices.length > 0) {
      flowEdges.push({ source: 'gw-bj', target: 'res-svc-0', action: 'allow', label: 'ACL 允许' });
    }

    // Security Score
    const hasDbAccess = subjectInfo.groups.includes('安全运维组');
    const securityScore = hasDbAccess ? 88 : 98; // Least privilege score

    return {
      subject: subjectInfo,
      stats: {
        accessibleSubnetsCount: accessibleSubnets.length,
        accessiblePortsCount: accessibleServices.length,
        accessibleDnsCount: accessibleDnsZones.length,
        deniedRulesCount: deniedAssets.length,
        securityScore,
      },
      accessibleSubnets,
      keyEndpoints: accessibleServices,
      dnsZones: accessibleDnsZones,
      deniedAssets,
      flowNodes,
      flowEdges,
    };
  },

  // Interactive Live Reachability Probe
  async probeAccessTarget(subjectType: 'group' | 'user' | 'device', subjectId: string, targetInput: string) {
    await delay(350);
    const target = targetInput.trim().toLowerCase();
    const matrix = await this.getAccessMatrix(subjectType, subjectId);

    if (matrix.subject.isBlocked) {
      return {
        allowed: false,
        target,
        statusLabel: '阻断拦截 (BLOCKED)',
        reason: matrix.subject.blockReason,
        matchedPolicy: '系统安全熔断策略',
        gateway: '无 (隧道未建立)',
        latency: 0,
      };
    }

    // Check if target is in denied ports
    if (target.includes(':23') || target.includes(':21')) {
      return {
        allowed: false,
        target,
        statusLabel: '策略显式拒绝 (DENY)',
        reason: '命中高危非标明文协议阻断策略 (Port 21/23)',
        matchedPolicy: '全网禁用明文 Telnet 与非标管理端口',
        gateway: '北京核心网关 (iptables REJECT)',
        latency: 2,
      };
    }

    // Check DB ports
    if ((target.includes('3306') || target.includes('5432') || target.includes('6379')) && !matrix.subject.groups.includes('安全运维组')) {
      return {
        allowed: false,
        target,
        statusLabel: '未授权阻断 (ZERO TRUST DENIED)',
        reason: '生产数据库集群端口仅限 [安全运维组] 接入，当前主体未获授权',
        matchedPolicy: '生产数据库专网只允许运维组接入',
        gateway: '北京核心网关 (DROP)',
        latency: 4,
      };
    }

    // Allow path
    return {
      allowed: true,
      target,
      statusLabel: '允许通行 (ALLOWED)',
      reason: '命中微隔离通行策略，WireGuard 路由表与 iptables Ingress 链均放行',
      matchedPolicy: matrix.subject.groups.includes('安全运维组')
        ? '生产数据库专网只允许运维组接入 / 默认跨站点安全互通'
        : '研发组访问上海及北京测试集群 / 默认跨站点安全互通',
      gateway: target.includes('.20.') ? 'gw-sh-branch-01 (上海研发网关)' : 'gw-bj-hub-01 (北京核心网关)',
      latency: 8,
    };
  },

  // --- 虚拟网络与 IPAM 自动化分配 (Overlay & IPAM Engine) ---
  async getOverlayNetwork() {
    await delay();
    const usedIps = ipamPoolsStore.reduce((acc, p) => acc + p.used, 0);
    const totalIps = 65534; // standard 10.100.0.0/16 host count
    return {
      global_cidr: '10.100.0.0/16',
      total_ips: totalIps,
      used_ips: usedIps,
      free_ips: totalIps - usedIps,
      ipam_pools: [...ipamPoolsStore],
      active_leases: [...ipamLeasesStore],
    };
  },

  async getIpamPools() {
    await delay();
    return [...ipamPoolsStore];
  },

  async getIpamPool(id: string) {
    await delay();
    const pool = ipamPoolsStore.find((p) => p.id === id);
    if (!pool) throw new Error('子网池未找到');
    return { ...pool };
  },

  async createIpamPool(data: Omit<IpamPool, 'id' | 'used'>) {
    await delay();
    const newPool: IpamPool = {
      ...data,
      id: `pool-${Date.now()}`,
      used: 0,
    };
    ipamPoolsStore.push(newPool);
    return newPool;
  },

  async updateIpamPool(id: string, data: Partial<IpamPool>) {
    await delay();
    ipamPoolsStore = ipamPoolsStore.map((p) => (p.id === id ? { ...p, ...data } : p));
    return ipamPoolsStore.find((p) => p.id === id)!;
  },

  async deleteIpamPool(id: string) {
    await delay();
    const pool = ipamPoolsStore.find((p) => p.id === id);
    if (!pool) throw new Error('子网池不存在');
    if (pool.used > 0) {
      throw new Error(`无法删除：该子网池当前仍有 ${pool.used} 个活跃 IP 租约，请先回收！`);
    }
    ipamPoolsStore = ipamPoolsStore.filter((p) => p.id !== id);
    return true;
  },

  async getIpamLeases(filter?: { search?: string; poolId?: string; status?: string; nodeType?: string }) {
    await delay();
    let leases = [...ipamLeasesStore];
    if (filter?.search) {
      const q = filter.search.toLowerCase();
      leases = leases.filter(
        (l) =>
          l.virtual_ip.toLowerCase().includes(q) ||
          l.target_name.toLowerCase().includes(q) ||
          l.wireguard_pubkey.toLowerCase().includes(q) ||
          l.endpoint.toLowerCase().includes(q)
      );
    }
    if (filter?.poolId && filter.poolId !== 'all') {
      leases = leases.filter((l) => l.pool_id === filter.poolId);
    }
    if (filter?.status && filter.status !== 'all') {
      leases = leases.filter((l) => l.status === filter.status);
    }
    if (filter?.nodeType && filter.nodeType !== 'all') {
      leases = leases.filter((l) => l.node_type === filter.nodeType);
    }
    return leases;
  },

  // 核心自动化分配引擎：计算下一个未被占用的 IP，锁定并绑定 WireGuard 公钥与硬件指纹
  async allocateIp(request: {
    target_name: string;
    node_type: 'Gateway' | 'Peer' | 'Server';
    pool_id: string;
    idp_group?: string;
    wireguard_pubkey?: string;
    endpoint?: string;
    notes?: string;
  }) {
    await delay(450);
    const pool = ipamPoolsStore.find((p) => p.id === request.pool_id);
    if (!pool) throw new Error('指定的虚拟子网池不存在');

    // Parse base subnet prefix from pool CIDR (e.g. 10.100.10.0/20 -> 10.100.10)
    const cidrParts = pool.cidr.split('/')[0].split('.');
    const basePrefix = `${cidrParts[0]}.${cidrParts[1]}.${cidrParts[2]}`;

    // Find all currently occupied last octets in this pool
    const existingIps = new Set(ipamLeasesStore.map((l) => l.virtual_ip));
    let assignedIp = '';

    for (let host = 1; host < 254; host++) {
      const candidateIp = `${basePrefix}.${host}`;
      if (!existingIps.has(candidateIp)) {
        assignedIp = candidateIp;
        break;
      }
    }

    if (!assignedIp) {
      // Fallback pseudo allocation
      assignedIp = `${basePrefix}.${Math.floor(Math.random() * 200) + 50}`;
    }

    // Generate random mock WireGuard pubkey if empty
    const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
    let generatedKey = request.wireguard_pubkey?.trim();
    if (!generatedKey || generatedKey.length < 20) {
      let rand = '';
      for (let i = 0; i < 43; i++) {
        rand += chars.charAt(Math.floor(Math.random() * chars.length));
      }
      generatedKey = `${rand}=`;
    }

    // Determine allocation mode
    let allocMode: '自动化分配-固定' | '飞书触发-动态锁定' | '钉钉触发-动态锁定' | '管理员手动保留' = '自动化分配-固定';
    if (request.node_type === 'Gateway') {
      allocMode = '自动化分配-固定';
    } else if (request.idp_group?.includes('飞书')) {
      allocMode = '飞书触发-动态锁定';
    } else if (request.idp_group?.includes('钉钉')) {
      allocMode = '钉钉触发-动态锁定';
    } else if (request.node_type === 'Server') {
      allocMode = '管理员手动保留';
    }

    const newLease = {
      id: `lease-${Date.now()}`,
      virtual_ip: assignedIp,
      node_type: request.node_type,
      target_name: request.target_name,
      endpoint: request.endpoint || `${Math.floor(Math.random() * 150) + 50}.${Math.floor(Math.random() * 200) + 10}.12.${Math.floor(Math.random() * 200) + 10}:51820`,
      allocation_mode: allocMode,
      pool_id: pool.id,
      pool_name: pool.pool_name,
      status: 'Active' as const,
      wireguard_pubkey: generatedKey,
      last_handshake: new Date().toISOString(),
      allocated_at: new Date().toISOString(),
    };

    // Update state
    ipamLeasesStore.unshift(newLease);
    pool.used += 1;

    // Add audit log
    auditStore.unshift({
      id: `audit-${Date.now()}`,
      timestamp: new Date().toISOString(),
      user: 'IPAM-Engine',
      device: request.target_name,
      node: 'gw-bj-hub-01',
      sourceIp: '0.0.0.0',
      destIp: assignedIp,
      port: 51820,
      action: 'allow',
      bytes: 0,
    });

    const wgInterfaceConfig = `[Interface]
# 自动生成的客户端配置 (VeloTrust IPAM Engine)
# 客户端名称: ${request.target_name}
# 分配模式: ${allocMode}
PrivateKey = <CLIENT_PRIVATE_KEY_HERE>
Address = ${assignedIp}/32
DNS = 10.100.0.1, 10.100.250.1

[Peer]
# 北京核心骨干网关 (Primary Hub)
PublicKey = aJ9uQ0kL8vM7nW2xY1zP4qR3sT6uV5wX8yZ0aB1cD2e=
Endpoint = 202.108.22.5:51820
AllowedIPs = 10.100.0.0/16, 192.168.10.0/24, 192.168.20.0/24
PersistentKeepalive = 25`;

    return {
      allocated_ip: assignedIp,
      lease: newLease,
      wg_interface_config: wgInterfaceConfig,
      gateway_vip: '10.100.0.1',
      dns_server: '10.100.250.1',
    };
  },

  // 释放/回收 IP 租约
  async releaseIpLease(leaseId: string) {
    await delay(300);
    const lease = ipamLeasesStore.find((l) => l.id === leaseId);
    if (!lease) throw new Error('租约未找到');

    const pool = ipamPoolsStore.find((p) => p.id === lease.pool_id);
    if (pool && pool.used > 0) {
      pool.used -= 1;
    }

    ipamLeasesStore = ipamLeasesStore.filter((l) => l.id !== leaseId);

    // Audit
    auditStore.unshift({
      id: `audit-${Date.now()}`,
      timestamp: new Date().toISOString(),
      user: 'admin',
      device: lease.target_name,
      node: 'gw-bj-hub-01',
      sourceIp: lease.virtual_ip,
      destIp: '0.0.0.0',
      port: 51820,
      action: 'allow',
      bytes: 0,
    });

    return true;
  },

  // 网段冲突检测与合规校验
  async checkIpConflict(cidrInput: string) {
    await delay(250);
    const cidr = cidrInput.trim();

    // Known physical / cloud networks
    const knownNetworks = [
      { name: 'Overlay 全局大网段', cidr: '10.100.0.0/16', type: 'overlay' },
      { name: '阿里云华东 VPC', cidr: '172.16.0.0/12', type: 'cloud_vpc' },
      { name: '腾讯云华南 VPC', cidr: '10.200.0.0/16', type: 'cloud_vpc' },
      { name: '北京总部内网', cidr: '192.168.10.0/24', type: 'physical_lan' },
      { name: '上海研发中心内网', cidr: '192.168.20.0/24', type: 'physical_lan' },
    ];

    if (!cidr.includes('/')) {
      return {
        isValid: false,
        hasConflict: false,
        message: '无效的 CIDR 格式，必须包含斜杠掩码 (如 10.100.60.0/24)',
      };
    }

    // Check if it starts with 10.100.
    const isWithinOverlay = cidr.startsWith('10.100.');
    const overlapsCloud = cidr.startsWith('172.16.') || cidr.startsWith('192.168.') || cidr.startsWith('10.200.');

    if (overlapsCloud) {
      return {
        isValid: true,
        hasConflict: true,
        conflictTarget: '企业既有云 VPC 或数据中心局域网',
        message: `严重警告：规划网段 ${cidr} 与现存物理/云 VPC 产生 IP 路由重叠！WireGuard 客户端将出现非对称路由或丢包，必须重新规划。`,
      };
    }

    // Check if it overlaps with existing pool in Overlay
    const conflictPool = ipamPoolsStore.find((p) => p.cidr === cidr);
    if (conflictPool) {
      return {
        isValid: true,
        hasConflict: true,
        conflictTarget: conflictPool.pool_name,
        message: `冲突：该网段已被 [${conflictPool.pool_name}] (${conflictPool.cidr}) 占用！`,
      };
    }

    return {
      isValid: true,
      hasConflict: false,
      isWithinOverlay,
      message: isWithinOverlay
        ? `合规通过：网段 ${cidr} 完全落入企业全局 Overlay (10.100.0.0/16) 内，且无任何物理/云网段重叠，可安全划分为新业务子网池。`
        : `提示：网段 ${cidr} 不在 10.100.0.0/16 内，如作为外部路由接入需确保所有网关节点同步更新路由表。`,
    };
  },

  // --- 网关离线逃生与兜底策略 (Fail-Safe Modes) ---
  async getFailSafeConfig(): Promise<FailSafeConfig> {
    await delay(150);
    return { ...failSafeConfigStore };
  },

  async updateFailSafeConfig(config: Partial<FailSafeConfig>): Promise<FailSafeConfig> {
    await delay(250);
    failSafeConfigStore = {
      ...failSafeConfigStore,
      ...config,
    };
    auditStore.unshift({
      id: 'aud-' + Date.now(),
      timestamp: new Date().toISOString(),
      user: 'admin (系统管理员)',
      device: 'MacBook-M3Max',
      node: 'gw-control-plane',
      sourceIp: '10.100.0.1',
      destIp: 'all-gateways',
      port: 51820,
      action: 'allow',
      bytes: 1024,
    });
    return { ...failSafeConfigStore };
  },

  async triggerFailSafeDrill(): Promise<{ success: boolean; log: string }> {
    await delay(600);
    const drillTime = new Date().toLocaleString();
    failSafeConfigStore.lastDrillRun = `${drillTime} (断网逃生演练成功)`;
    return {
      success: true,
      log: `[${drillTime}] 模拟中央控制大脑完全宕机：向 5 台分布式网关 Agent 发送模拟断网心跳。网关成功平滑切入 [${failSafeConfigStore.mode}] 模式，保持现有 ${ipamLeasesStore.length} 个 WireGuard 本地 Peer 路由连接通畅，应急管理员通道就绪。`,
    };
  },

  // --- 跨云重叠网段 NAT 映射 (VPC Overlap NAT) ---
  async updateSiteNatMapping(
    siteId: string,
    natMapping: { enabled: boolean; virtualCidr: string; natType: '1:1_Bidirectional' | 'SNAT_Masquerade' | 'DNAT_VirtualIP'; description?: string }
  ): Promise<Site> {
    await delay(250);
    const siteIdx = sitesStore.findIndex((s) => s.id === siteId);
    if (siteIdx === -1) throw new Error('站点不存在');

    sitesStore[siteIdx] = {
      ...sitesStore[siteIdx],
      natMapping: {
        ...natMapping,
        mappedAt: natMapping.enabled ? new Date().toISOString() : undefined,
      },
    };

    return { ...sitesStore[siteIdx] };
  },

  // --- 角色与控制台权限体系 (Role & RBAC Management) ---
  async getRoles(): Promise<Role[]> {
    await delay(120);
    return [...rolesStore];
  },

  async getRole(id: string): Promise<Role | null> {
    await delay(100);
    return rolesStore.find((r) => r.id === id) || null;
  },

  async getAllPermissions(): Promise<PermissionItem[]> {
    await delay(80);
    return [...allPermissions];
  },

  async saveRole(roleData: Partial<Role> & { name: string; code: string }): Promise<Role> {
    await delay(200);
    if (roleData.id) {
      const idx = rolesStore.findIndex((r) => r.id === roleData.id);
      if (idx !== -1) {
        rolesStore[idx] = {
          ...rolesStore[idx],
          ...roleData,
          updatedAt: new Date().toISOString(),
        } as Role;
        auditStore.unshift({
          id: 'aud-' + Date.now(),
          timestamp: new Date().toISOString(),
          user: '当前管理员',
          device: 'Web 控制台',
          node: 'console-rbac',
          sourceIp: '127.0.0.1',
          destIp: 'rbac://roles/' + roleData.code,
          port: 443,
          action: 'allow',
          bytes: 512,
        });
        return { ...rolesStore[idx] };
      }
    }
    const newRole: Role = {
      id: 'role-' + Date.now(),
      name: roleData.name,
      code: roleData.code,
      type: roleData.type || 'custom',
      separationType: roleData.separationType || 'custom',
      description: roleData.description || '',
      userCount: 0,
      menuRoutes: roleData.menuRoutes || ['/'],
      permissionCodes: roleData.permissionCodes || [],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    rolesStore.push(newRole);
    return { ...newRole };
  },

  async deleteRole(id: string): Promise<boolean> {
    await delay(150);
    const target = rolesStore.find((r) => r.id === id);
    if (!target) return false;
    if (target.type === 'system') {
      throw new Error('系统内置核心角色不可直接删除，以保障三权分立与控制台底线安全！');
    }
    rolesStore = rolesStore.filter((r) => r.id !== id);
    return true;
  },

  // --- 受保护业务资源目录 (Protected Business Resources) ---
  async getProtectedResources(): Promise<ProtectedResource[]> {
    await delay(150);
    return [...protectedResourcesStore];
  },

  async getProtectedResource(id: string): Promise<ProtectedResource | null> {
    await delay(100);
    return protectedResourcesStore.find((r) => r.id === id) || null;
  },

  async saveProtectedResource(data: Partial<ProtectedResource> & { name: string; host: string }): Promise<ProtectedResource> {
    await delay(200);
    if (data.id) {
      const idx = protectedResourcesStore.findIndex((r) => r.id === data.id);
      if (idx !== -1) {
        protectedResourcesStore[idx] = {
          ...protectedResourcesStore[idx],
          ...data,
        } as ProtectedResource;
        return { ...protectedResourcesStore[idx] };
      }
    }
    const newRes: ProtectedResource = {
      id: 'res-' + Date.now(),
      name: data.name,
      code: data.code || 'SVC-' + Math.floor(1000 + Math.random() * 9000),
      category: data.category || 'web_portal',
      protocol: data.protocol || 'HTTPS',
      host: data.host,
      port: data.port || 443,
      url: data.url,
      siteId: data.siteId || 'site-bj',
      siteName: data.siteName || '北京总部核心区 (阿里云 VPC)',
      gatewayNodeId: data.gatewayNodeId || 'node-hub-bj',
      gatewayNodeName: data.gatewayNodeName || 'gw-bj-hub-01',
      sensitivity: data.sensitivity || 'Internal',
      department: data.department || '通用研发部',
      owner: data.owner || '管理员',
      status: 'healthy',
      latencyMs: Math.floor(8 + Math.random() * 25),
      healthCheckType: data.healthCheckType || 'HTTP_GET',
      associatedPoliciesCount: 1,
      tags: data.tags || ['新增受保护资产'],
      description: data.description || '',
      createdAt: new Date().toISOString(),
    };
    protectedResourcesStore.unshift(newRes);
    return { ...newRes };
  },

  async deleteProtectedResource(id: string): Promise<boolean> {
    await delay(150);
    protectedResourcesStore = protectedResourcesStore.filter((r) => r.id !== id);
    return true;
  },

  async probeResourceHealth(id: string): Promise<{
    success: boolean;
    latencyMs: number;
    statusText: string;
    details: string;
  }> {
    await delay(400);
    const target = protectedResourcesStore.find((r) => r.id === id);
    if (!target) throw new Error('资源不存在');
    const isDegraded = target.status === 'degraded';
    const latency = isDegraded ? Math.floor(80 + Math.random() * 60) : Math.floor(5 + Math.random() * 15);
    target.latencyMs = latency;
    return {
      success: true,
      latencyMs: latency,
      statusText: isDegraded ? '响应较慢 (Degraded)' : '健康连接 (Healthy)',
      details: `经由网关 [${target.gatewayNodeName}] 发送 ${target.healthCheckType} 握手到 ${target.host}:${target.port}。往返时延 ${latency}ms，未发现丢包与异常 reset。`,
    };
  },

  // --- 菜单管理 (Menu Management API) ---
  async getMenus(): Promise<SysMenu[]> {
    await delay(120);
    return [...menusStore];
  },

  async getMenu(id: string): Promise<SysMenu | undefined> {
    await delay(80);
    const item = menusStore.find((m) => m.id === id);
    return item ? { ...item } : undefined;
  },

  async saveMenu(data: Partial<SysMenu>): Promise<SysMenu> {
    await delay(150);
    if (data.id) {
      const idx = menusStore.findIndex((m) => m.id === data.id);
      if (idx !== -1) {
        menusStore[idx] = {
          ...menusStore[idx],
          ...data,
        } as SysMenu;
        return { ...menusStore[idx] };
      }
    }
    const newMenu: SysMenu = {
      id: 'menu-' + Date.now(),
      parentId: data.parentId || null,
      title: data.title || '新菜单',
      path: data.path || '',
      component: data.component,
      icon: data.icon || 'SlidersHorizontal',
      sortOrder: data.sortOrder || menusStore.length + 1,
      type: data.type || 'menu',
      permissionCode: data.permissionCode || 'menu:custom',
      visible: data.visible !== false,
      status: data.status || 'active',
      badge: data.badge,
      isExternal: !!data.isExternal,
      createdAt: new Date().toISOString(),
    };
    menusStore.push(newMenu);
    return { ...newMenu };
  },

  async deleteMenu(id: string): Promise<boolean> {
    await delay(120);
    // Delete target and any child menus
    menusStore = menusStore.filter((m) => m.id !== id && m.parentId !== id);
    return true;
  },

  // --- 权限管理 (Permission Management API) ---
  async getPermissions(): Promise<SysPermission[]> {
    await delay(120);
    // Dynamically calculate how many APIs are mapped to each permission
    return permissionsStore.map((p) => {
      const count = apiRpcMappingsStore.filter((m) => m.mappedPermissionCode === p.code).length;
      return {
        ...p,
        mappedApisCount: count,
      };
    });
  },

  async getPermission(id: string): Promise<SysPermission | undefined> {
    await delay(80);
    const item = permissionsStore.find((p) => p.id === id);
    if (!item) return undefined;
    const count = apiRpcMappingsStore.filter((m) => m.mappedPermissionCode === item.code).length;
    return { ...item, mappedApisCount: count };
  },

  async savePermission(data: Partial<SysPermission>): Promise<SysPermission> {
    await delay(150);
    if (data.id) {
      const idx = permissionsStore.findIndex((p) => p.id === data.id);
      if (idx !== -1) {
        permissionsStore[idx] = {
          ...permissionsStore[idx],
          ...data,
        } as SysPermission;
        return { ...permissionsStore[idx] };
      }
    }
    const newPerm: SysPermission = {
      id: 'perm-' + Date.now(),
      code: data.code || `${data.resourceCode || 'custom'}:${data.action || 'read'}`,
      name: data.name || `${data.resourceName || '自定义资源'} · ${data.actionLabel || data.action || '读取'}`,
      type: data.type || 'api',
      resourceCode: data.resourceCode || 'user',
      resourceName: data.resourceName || '用户管理',
      action: data.action || 'read',
      actionLabel: data.actionLabel || '读取 (read)',
      isSensitive: data.isSensitive || false,
      module: data.module || 'identity',
      dataScope: data.dataScope,
      description: data.description || '',
      status: data.status || 'active',
      assignedRolesCount: data.assignedRolesCount || 1,
      assignedRoles: data.assignedRoles || ['超级管理员'],
      mappedApisCount: 0,
      createdAt: new Date().toISOString(),
    };
    permissionsStore.unshift(newPerm);
    return { ...newPerm };
  },

  async deletePermission(id: string): Promise<boolean> {
    await delay(120);
    const target = permissionsStore.find((p) => p.id === id);
    if (target) {
      // Unlink mapped APIs
      apiRpcMappingsStore.forEach((m) => {
        if (m.mappedPermissionCode === target.code) {
          m.mappedPermissionCode = null;
          m.mappedResourceCode = null;
          m.mappedAction = null;
          m.status = 'unmapped';
        }
      });
    }
    permissionsStore = permissionsStore.filter((p) => p.id !== id);
    return true;
  },

  // --- 1. 业务资源对象管理 (Business Authorization Resources API) ---
  async getAuthResources(): Promise<AuthResource[]> {
    await delay(120);
    return [...authResourcesStore];
  },

  async saveAuthResource(data: Partial<AuthResource>): Promise<AuthResource> {
    await delay(150);
    if (data.id) {
      const idx = authResourcesStore.findIndex((r) => r.id === data.id);
      if (idx !== -1) {
        authResourcesStore[idx] = {
          ...authResourcesStore[idx],
          ...data,
        } as AuthResource;
        return { ...authResourcesStore[idx] };
      }
    }
    const newRes: AuthResource = {
      id: 'res-' + (data.code || Date.now()),
      code: data.code || 'custom_resource',
      name: data.name || '新建业务资源',
      category: data.category || 'security',
      description: data.description || '',
      status: data.status || 'active',
      supportedActions: data.supportedActions || [
        { action: 'read', label: '读取', description: '查询、查看详情、获取列表' },
        { action: 'write', label: '写入', description: '创建、修改、删除等数据变更操作' },
      ],
      createdAt: new Date().toISOString(),
    };
    authResourcesStore.unshift(newRes);
    return { ...newRes };
  },

  async deleteAuthResource(id: string): Promise<boolean> {
    await delay(120);
    authResourcesStore = authResourcesStore.filter((r) => r.id !== id);
    return true;
  },

  // --- 3. 后端 API/RPC 接口映射管理 (API & RPC Mappings API) ---
  async getApiMappings(): Promise<ApiRpcMapping[]> {
    await delay(120);
    return [...apiRpcMappingsStore];
  },

  async getApiResources(): Promise<ApiResource[]> {
    await delay(120);
    return [...apiRpcMappingsStore];
  },

  async saveApiMapping(data: Partial<ApiRpcMapping>): Promise<ApiRpcMapping> {
    await delay(150);
    if (data.id) {
      const idx = apiRpcMappingsStore.findIndex((r) => r.id === data.id);
      if (idx !== -1) {
        const updated = {
          ...apiRpcMappingsStore[idx],
          ...data,
          status: data.mappedPermissionCode ? 'mapped' : 'unmapped',
        } as ApiRpcMapping;
        apiRpcMappingsStore[idx] = updated;
        return { ...updated };
      }
    }
    const newApi: ApiRpcMapping = {
      id: 'api-' + Date.now(),
      code: data.code || 'custom.service.NewMethod',
      name: data.name || '未命名 API 接口',
      type: data.type || 'HTTP_API',
      path: data.path || '/api/v1/custom/endpoint',
      method: data.method || 'GET',
      serviceModule: data.serviceModule || 'identity',
      rateLimit: data.rateLimit || '1000 req/min',
      authRequired: data.authRequired !== false,
      riskLevel: data.riskLevel || 'Medium',
      mappedPermissionCode: data.mappedPermissionCode || null,
      mappedResourceCode: data.mappedResourceCode || null,
      mappedAction: data.mappedAction || null,
      status: data.mappedPermissionCode ? 'mapped' : 'unmapped',
      description: data.description || '',
      createdAt: new Date().toISOString(),
    };
    apiRpcMappingsStore.unshift(newApi);
    return { ...newApi };
  },

  async batchMapApis(
    apiIds: string[],
    permissionCode: string,
    resourceCode?: string,
    action?: string
  ): Promise<number> {
    await delay(200);
    let count = 0;
    apiRpcMappingsStore = apiRpcMappingsStore.map((item) => {
      if (apiIds.includes(item.id)) {
        count++;
        return {
          ...item,
          mappedPermissionCode: permissionCode,
          mappedResourceCode: resourceCode || permissionCode.split(':')[0],
          mappedAction: action || permissionCode.split(':')[1] || 'read',
          status: 'mapped',
        };
      }
      return item;
    });
    return count;
  },

  async deleteApiMapping(id: string): Promise<boolean> {
    await delay(120);
    apiRpcMappingsStore = apiRpcMappingsStore.filter((r) => r.id !== id);
    return true;
  },

  async saveApiResource(data: Partial<ApiResource>): Promise<ApiResource> {
    return this.saveApiMapping(data);
  },

  async deleteApiResource(id: string): Promise<boolean> {
    return this.deleteApiMapping(id);
  },

  async syncBackendApis(): Promise<{ total: number; newlyDiscovered: number }> {
    await delay(600);
    // Simulates syncing with backend proto/swagger
    const discovered = [
      {
        id: 'api-sync-' + Date.now() + '-1',
        code: 'sec.risk.GetPostureScore',
        name: '获取实时主机安全态势评分',
        type: 'HTTP_API' as const,
        method: 'GET' as const,
        path: '/api/v1/security/posture-score',
        serviceModule: 'security' as const,
        rateLimit: '3000 req/min',
        authRequired: true,
        riskLevel: 'Medium' as const,
        mappedPermissionCode: null,
        mappedResourceCode: null,
        mappedAction: null,
        status: 'unmapped' as const,
        description: '【新同步】从分布式端点收集健康基线评分与漏洞指标，待映射至资源操作权限。默认拒绝！',
        createdAt: new Date().toISOString(),
      },
    ];
    apiRpcMappingsStore.unshift(...discovered);
    return {
      total: apiRpcMappingsStore.length,
      newlyDiscovered: discovered.length,
    };
  },

  // --- 4. 接口鉴权流程仿真测试器 (Authorization Simulator) ---
  async simulateAuthorization(roleCode: string, apiCodeOrPath: string): Promise<{
    allowed: boolean;
    decisionCode: 'ALLOWED' | 'DENIED_UNMAPPED_DEFAULT_DENY' | 'DENIED_NO_PERMISSION';
    apiEndpoint: ApiRpcMapping | null;
    requiredPermission: SysPermission | null;
    role: Role | null;
    traceSteps: {
      step: number;
      title: string;
      status: 'pass' | 'fail' | 'warn' | 'info';
      detail: string;
    }[];
  }> {
    await delay(300);
    const role = rolesStore.find((r) => r.code === roleCode) || null;
    const api =
      apiRpcMappingsStore.find((m) => m.code === apiCodeOrPath || m.path === apiCodeOrPath || m.id === apiCodeOrPath) ||
      null;

    const traceSteps: {
      step: number;
      title: string;
      status: 'pass' | 'fail' | 'warn' | 'info';
      detail: string;
    }[] = [];

    traceSteps.push({
      step: 1,
      title: '识别请求后端接口',
      status: api ? 'pass' : 'fail',
      detail: api
        ? `请求定位至 [${api.type}] ${api.method} ${api.path} (${api.code})`
        : `未在接口目录中找到标识为 "${apiCodeOrPath}" 的接口定义`,
    });

    if (!api) {
      return {
        allowed: false,
        decisionCode: 'DENIED_UNMAPPED_DEFAULT_DENY',
        apiEndpoint: null,
        requiredPermission: null,
        role,
        traceSteps,
      };
    }

    traceSteps.push({
      step: 2,
      title: '检索接口到资源操作权限的映射',
      status: api.mappedPermissionCode ? 'pass' : 'warn',
      detail: api.mappedPermissionCode
        ? `接口映射到资源操作权限：[${api.mappedPermissionCode}] (所属资源: ${api.mappedResourceCode}, 操作类型: ${api.mappedAction})`
        : `⚠️ 接口尚未映射至任何资源操作权限！触发底层安全策略：默认拒绝 (Default Deny)`,
    });

    if (!api.mappedPermissionCode) {
      traceSteps.push({
        step: 3,
        title: '执行底线安全策略 (Default Deny)',
        status: 'fail',
        detail: `安全阻断：未映射接口一律默认拒绝，防止开发新增接口遗漏配置引发未授权越权访问漏洞。`,
      });
      return {
        allowed: false,
        decisionCode: 'DENIED_UNMAPPED_DEFAULT_DENY',
        apiEndpoint: api,
        requiredPermission: null,
        role,
        traceSteps,
      };
    }

    const requiredPerm = permissionsStore.find((p) => p.code === api.mappedPermissionCode) || null;

    traceSteps.push({
      step: 3,
      title: '核验用户激活角色及权限集',
      status: role ? 'pass' : 'fail',
      detail: role
        ? `用户携带角色 [${role.name} (${role.code})]，已分配 ${role.permissionCodes.length} 项资源操作权限`
        : `未指定有效角色凭证`,
    });

    if (!role) {
      return {
        allowed: false,
        decisionCode: 'DENIED_NO_PERMISSION',
        apiEndpoint: api,
        requiredPermission: requiredPerm,
        role: null,
        traceSteps,
      };
    }

    const hasPerm = role.permissionCodes.includes(api.mappedPermissionCode);

    traceSteps.push({
      step: 4,
      title: 'RBAC 资源权限判定决策',
      status: hasPerm ? 'pass' : 'fail',
      detail: hasPerm
        ? `✅ 角色 [${role.name}] 拥有所需的 [${api.mappedPermissionCode}] 权限，鉴权通过允许执行！`
        : `🚫 角色 [${role.name}] 缺少所需的 [${api.mappedPermissionCode}] 权限，鉴权拦截返回 403 Forbidden。`,
    });

    return {
      allowed: hasPerm,
      decisionCode: hasPerm ? 'ALLOWED' : 'DENIED_NO_PERMISSION',
      apiEndpoint: api,
      requiredPermission: requiredPerm,
      role,
      traceSteps,
    };
  },

  // --- 登录审计日志 (Login Logs API) ---
  async getLoginLogs(): Promise<LoginLog[]> {
    await delay(120);
    return [...loginLogsStore];
  },

  async addLoginLog(data: Partial<LoginLog>): Promise<LoginLog> {
    await delay(80);
    const newLog: LoginLog = {
      id: 'log-login-' + Date.now(),
      timestamp: data.timestamp || new Date().toISOString(),
      username: data.username || 'admin',
      name: data.name || '管理员',
      loginType: data.loginType || 'password',
      status: data.status || 'success',
      ip: data.ip || '192.168.1.100',
      location: data.location || '北京 · 海淀区',
      device: data.device || 'Chrome / macOS',
      browser: data.browser || 'Chrome 128.0',
      message: data.message || '通过控制台登录认证',
      riskScore: data.riskScore !== undefined ? data.riskScore : 10,
      sessionId: 'sess_' + Math.random().toString(36).substring(2, 10),
      mfaUsed: data.mfaUsed !== false,
    };
    loginLogsStore.unshift(newLog);
    return { ...newLog };
  },

  async clearLoginLogs(): Promise<boolean> {
    await delay(150);
    loginLogsStore = [];
    return true;
  },

  // --- 操作审计日志 (Operation Audit Logs API) ---
  async getOperationLogs(): Promise<OperationLog[]> {
    await delay(120);
    return [...operationLogsStore];
  },

  async addOperationLog(data: Partial<OperationLog>): Promise<OperationLog> {
    await delay(80);
    const newLog: OperationLog = {
      id: 'oplog-' + Date.now(),
      timestamp: data.timestamp || new Date().toISOString(),
      operatorUsername: data.operatorUsername || 'admin',
      operatorName: data.operatorName || '超级管理员 (张伟)',
      operatorRole: data.operatorRole || '超级管理员',
      module: data.module || '访问控制策略',
      actionType: data.actionType || 'CREATE',
      actionName: data.actionName || '执行控制台管理操作',
      targetResource: data.targetResource || 'res-custom-' + Math.floor(Math.random() * 100),
      requestMethod: data.requestMethod || 'POST',
      requestUri: data.requestUri || '/api/v1/admin/operation',
      ip: data.ip || '192.168.1.100',
      location: data.location || '北京 · 总部研发中心',
      device: data.device || 'MacBook Pro / Chrome 128',
      status: data.status || 'SUCCESS',
      durationMs: data.durationMs || Math.floor(15 + Math.random() * 60),
      errorMessage: data.errorMessage,
      diffSnapshot: data.diffSnapshot || {
        after: {
          operatedAt: new Date().toISOString(),
          operator: data.operatorUsername || 'admin',
        },
      },
    };
    operationLogsStore.unshift(newLog);
    return { ...newLog };
  },

  async clearOperationLogs(): Promise<boolean> {
    await delay(150);
    operationLogsStore = [];
    return true;
  },

  // ================= 注册凭证管理 (Enrollment Tokens API) =================
  async getEnrollmentTokens(filter?: { status?: string; search?: string }): Promise<EnrollmentToken[]> {
    await delay(120);
    let list = [...enrollmentTokensStore];

    // Auto-update expired tokens based on current time
    const now = new Date().getTime();
    list = list.map((t) => {
      if (t.status === 'PENDING' && new Date(t.expiresAt).getTime() < now) {
        t.status = 'EXPIRED';
      }
      return t;
    });

    if (filter?.status && filter.status !== 'all') {
      list = list.filter((t) => t.status === filter.status);
    }
    if (filter?.search) {
      const q = filter.search.toLowerCase();
      list = list.filter(
        (t) =>
          t.nodeName.toLowerCase().includes(q) ||
          t.token.toLowerCase().includes(q) ||
          t.networkName.toLowerCase().includes(q) ||
          t.allowedSubnets.some((s) => s.toLowerCase().includes(q)) ||
          t.id.toLowerCase().includes(q)
      );
    }
    return list;
  },

  async getEnrollmentToken(id: string): Promise<EnrollmentToken> {
    await delay(80);
    const item = enrollmentTokensStore.find((t) => t.id === id);
    if (!item) throw new Error('注册凭证不存在或已被删除');
    return { ...item };
  },

  // 阶段一：管理员预先创建逻辑节点
  async preCreateLogicalNode(data: PreCreateNodeInput): Promise<Node> {
    await delay(150);
    const existing = nodesStore.find((n) => n.name.toLowerCase() === data.name.trim().toLowerCase());
    if (existing) {
      throw new Error(`节点名称 "${data.name}" 已存在，请使用唯一的节点名称`);
    }

    const newNodeId = `node-gw-${Date.now().toString(36)}`;
    const randomIpLast = Math.floor(15 + Math.random() * 200);
    const newNode: Node = {
      id: newNodeId,
      name: data.name.trim(),
      type: 'gateway',
      nodeTypeLabel: data.nodeTypeLabel || '站点接入网关',
      virtualIp: data.virtualIp || `10.0.0.${randomIpLast}/24`,
      siteId: data.siteId || 'site-sh',
      siteName: data.siteName || (data.networkName.includes('上海') ? '上海研发中心' : data.networkName.includes('北京') ? '北京总部核心区' : '通用接入站点'),
      networkName: data.networkName,
      allowedSubnets: data.allowedSubnets && data.allowedSubnets.length > 0 ? data.allowedSubnets : ['10.20.0.0/16'],
      nodeRoles: data.nodeRoles && data.nodeRoles.length > 0 ? data.nodeRoles : ['流量转发', '站点互联'],
      registrationMethod: 'token',
      requireApproval: data.requireApproval,
      enrollmentStatus: 'unregistered',
      status: 'pending_enrollment',
      lastHandshake: new Date().toISOString(),
      rxBytes: 0,
      txBytes: 0,
      endpoint: '等待节点通过一次性凭证入网',
      publicKey: 'PENDING_REGISTRATION_KEY',
      createdAt: new Date().toISOString(),
    };

    nodesStore.unshift(newNode);

    // Audit log
    operationLogsStore.unshift({
      id: 'oplog-' + Date.now(),
      timestamp: new Date().toISOString(),
      operatorUsername: 'admin',
      operatorName: '超级管理员 (张伟)',
      operatorRole: '超级管理员',
      module: '虚拟节点与网关',
      actionType: 'CREATE',
      actionName: `预先创建网关逻辑节点: ${newNode.name}`,
      targetResource: `节点: ${newNode.name} (${newNode.networkName})`,
      requestMethod: 'POST',
      requestUri: '/api/v1/nodes/pre-create',
      ip: '192.168.1.100',
      location: '北京 · 总部研发中心',
      device: 'MacBook Pro / Chrome 128',
      status: 'SUCCESS',
      durationMs: 25,
      diffSnapshot: {
        after: {
          nodeName: newNode.name,
          nodeType: newNode.nodeTypeLabel,
          networkName: newNode.networkName,
          allowedSubnets: newNode.allowedSubnets,
          nodeRoles: newNode.nodeRoles,
          registrationMethod: '一次性注册凭证',
          requireApproval: newNode.requireApproval,
        },
      },
    });

    return { ...newNode };
  },

  // 阶段二：控制面为该逻辑节点生成一次性注册凭证
  async generateEnrollmentToken(data: GenerateTokenInput): Promise<EnrollmentToken> {
    await delay(180);
    const node = nodesStore.find((n) => n.id === data.nodeId);
    if (!node) {
      throw new Error('未找到绑定的逻辑节点记录，请先在阶段一完成节点预创建');
    }

    // Check if there is already an active PENDING token for this node
    const existingActive = enrollmentTokensStore.find(
      (t) => t.nodeId === data.nodeId && t.status === 'PENDING' && new Date(t.expiresAt).getTime() > Date.now()
    );
    if (existingActive) {
      throw new Error(`节点 "${node.name}" 当前已存在有效待注册凭证 (${existingActive.id})，如需重新生成请先作废原凭证。`);
    }

    const randomHex = Array.from({ length: 32 }, () => Math.floor(Math.random() * 16).toString(16)).join('');
    const tokenString = `vt_tok_${randomHex}`;
    const tokenId = `tok-wg-${Date.now().toString(36)}`;
    const expiresHours = data.expiresInHours || 24;
    const expiresAt = new Date(Date.now() + 1000 * 3600 * expiresHours).toISOString();

    const newToken: EnrollmentToken = {
      id: tokenId,
      token: tokenString,
      nodeId: node.id,
      nodeName: node.name,
      nodeType: node.nodeTypeLabel || '站点接入网关',
      networkName: node.networkName || '默认网络',
      allowedSubnets: node.allowedSubnets || ['10.20.0.0/16'],
      nodeRoles: node.nodeRoles || ['流量转发', '站点互联'],
      registrationMethod: 'token',
      requireApproval: !!node.requireApproval,
      approvalStatus: node.requireApproval ? 'approved' : undefined,
      approvedBy: node.requireApproval ? 'sec_officer (安全管理员预审通过)' : undefined,
      approvedAt: node.requireApproval ? new Date().toISOString() : undefined,
      permissions: ['node:register_only'], // 严格不具备管理 API、访问业务资源等额外权限
      maxUsage: 1, // 默认只能使用一次
      usedCount: 0,
      status: 'PENDING',
      createdAt: new Date().toISOString(),
      createdBy: 'admin (系统管理员)',
      expiresAt: expiresAt,
      installCommand: `curl -fsSL https://get.velotrust.internal/gateway-agent.sh | bash -s -- --token ${tokenString} --server https://mesh-control.velotrust.internal --node ${node.name}`,
      configSnippet: `[Interface]\nPrivateKey = <AUTO_GENERATED_ON_NODE_BOOT>\nAddress = ${node.virtualIp}\nDNS = 10.100.250.1\n\n[Peer]\nPublicKey = aJ9uQ0kL8vM7nW2xY1zP4qR3sT6uV5wX8yZ0aB1cD2e=\nEndpoint = 202.108.22.5:51820\nAllowedIPs = ${(node.allowedSubnets || ['10.20.0.0/16']).join(', ')}\nPersistentKeepalive = 25`,
      notes: data.notes || `绑定节点 ${node.name}，授权网段 ${(node.allowedSubnets || []).join(';')}`,
    };

    enrollmentTokensStore.unshift(newToken);

    // Update node link
    nodesStore = nodesStore.map((n) => (n.id === node.id ? { ...n, enrollmentTokenId: tokenId } : n));

    // Audit log
    operationLogsStore.unshift({
      id: 'oplog-' + Date.now(),
      timestamp: new Date().toISOString(),
      operatorUsername: 'admin',
      operatorName: '超级管理员 (张伟)',
      operatorRole: '超级管理员',
      module: '网关注册凭证',
      actionType: 'GRANT',
      actionName: `为节点 ${node.name} 生成一次性入网凭证`,
      targetResource: `凭证: ${tokenId} (节点: ${node.name})`,
      requestMethod: 'POST',
      requestUri: '/api/v1/enrollment-tokens/generate',
      ip: '192.168.1.100',
      location: '北京 · 总部研发中心',
      device: 'MacBook Pro / Chrome 128',
      status: 'SUCCESS',
      durationMs: 32,
      diffSnapshot: {
        after: {
          tokenId,
          nodeName: node.name,
          expiresAt,
          maxUsage: 1,
          permissions: ['node:register_only (无额外权限)'],
        },
      },
    });

    return { ...newToken };
  },

  // 完整的两阶段串联创建 (阶段一预创建节点 + 阶段二生成凭证)
  async preCreateNodeAndToken(
    nodeData: PreCreateNodeInput,
    tokenData: { expiresInHours: number; notes?: string }
  ): Promise<{ node: Node; token: EnrollmentToken }> {
    const node = await this.preCreateLogicalNode(nodeData);
    const token = await this.generateEnrollmentToken({
      nodeId: node.id,
      expiresInHours: tokenData.expiresInHours || 24,
      notes: tokenData.notes,
    });
    return { node, token };
  },

  // 作废注册凭证
  async revokeEnrollmentToken(id: string, reason: string): Promise<EnrollmentToken> {
    await delay(120);
    const token = enrollmentTokensStore.find((t) => t.id === id);
    if (!token) throw new Error('凭证不存在');
    if (token.status === 'USED') throw new Error('已消耗入网的凭证无法作废');

    token.status = 'REVOKED';
    token.revokedAt = new Date().toISOString();
    token.revokedBy = 'admin (超级管理员)';
    token.revokeReason = reason || '管理员手动撤回凭证';

    operationLogsStore.unshift({
      id: 'oplog-' + Date.now(),
      timestamp: new Date().toISOString(),
      operatorUsername: 'admin',
      operatorName: '超级管理员 (张伟)',
      operatorRole: '超级管理员',
      module: '网关注册凭证',
      actionType: 'DISABLE',
      actionName: `作废一次性注册凭证: ${token.id}`,
      targetResource: `凭证: ${token.id} (节点: ${token.nodeName})`,
      requestMethod: 'POST',
      requestUri: `/api/v1/enrollment-tokens/${id}/revoke`,
      ip: '192.168.1.100',
      location: '北京 · 总部研发中心',
      device: 'MacBook Pro / Chrome 128',
      status: 'SUCCESS',
      durationMs: 20,
      diffSnapshot: {
        before: { status: 'PENDING' },
        after: { status: 'REVOKED', reason },
      },
    });

    return { ...token };
  },

  // 延长凭证有效期
  async extendEnrollmentToken(id: string, additionalHours: number): Promise<EnrollmentToken> {
    await delay(120);
    const token = enrollmentTokensStore.find((t) => t.id === id);
    if (!token) throw new Error('凭证不存在');
    if (token.status === 'USED') throw new Error('已使用的凭证不可延长');

    const currentExpiry = new Date(token.expiresAt).getTime();
    const baseTime = currentExpiry > Date.now() ? currentExpiry : Date.now();
    token.expiresAt = new Date(baseTime + additionalHours * 3600 * 1000).toISOString();
    if (token.status === 'EXPIRED') {
      token.status = 'PENDING';
    }

    return { ...token };
  },

  // 模拟节点入网握手 (网关使用一次性凭证入网，成功注册后立即失效)
  async simulateNodeEnrollment(
    tokenId: string,
    clientInfo?: { clientIp?: string; hostname?: string }
  ): Promise<{ success: boolean; message: string; node: Node; token: EnrollmentToken }> {
    await delay(600); // realistic network handshake delay
    const token = enrollmentTokensStore.find((t) => t.id === tokenId);
    if (!token) {
      throw new Error('入网失败：凭证不存在');
    }

    if (token.status === 'USED') {
      throw new Error(`安全拦截：一次性注册凭证已于 ${new Date(token.usedAt!).toLocaleString()} 使用并失效，拒绝重复入网！`);
    }

    if (token.status === 'REVOKED') {
      throw new Error(`安全拦截：注册凭证已被管理员作废（原因：${token.revokeReason}），拒绝入网请求！`);
    }

    if (new Date(token.expiresAt).getTime() < Date.now()) {
      token.status = 'EXPIRED';
      throw new Error('安全拦截：注册凭证已超过有效时间，请管理员重新生成入网凭证！');
    }

    const assignedIp = clientInfo?.clientIp || `180.168.41.${Math.floor(20 + Math.random() * 200)}`;
    const hostname = clientInfo?.hostname || `${token.nodeName}.mesh.internal`;

    // 1. Mark token as USED immediately (成功注册后立即失效)
    token.status = 'USED';
    token.usedCount = 1;
    token.usedAt = new Date().toISOString();
    token.usedIp = assignedIp;
    token.deviceFingerprint = `Linux 6.8.0-x86_64-Ubuntu (${hostname})`;

    // 2. Activate Node to online and registered
    const nodeIdx = nodesStore.findIndex((n) => n.id === token.nodeId);
    let updatedNode: Node;
    if (nodeIdx !== -1) {
      nodesStore[nodeIdx] = {
        ...nodesStore[nodeIdx],
        status: 'online',
        enrollmentStatus: 'registered',
        lastHandshake: new Date().toISOString(),
        endpoint: `${assignedIp}:51820`,
        publicKey: `WgPeerKey_${Math.random().toString(36).substring(2, 12)}_Pub=`,
        rxBytes: 1024,
        txBytes: 2048,
      };
      updatedNode = nodesStore[nodeIdx];
    } else {
      throw new Error('未找到关联的节点信息');
    }

    // 3. Record Audit Log for successful enrollment & token destruction
    operationLogsStore.unshift({
      id: 'oplog-' + Date.now(),
      timestamp: new Date().toISOString(),
      operatorUsername: 'system:gateway-agent',
      operatorName: `网关接入节点 (${token.nodeName})`,
      operatorRole: '网关入网凭据',
      module: '网关注册凭证',
      actionType: 'GRANT',
      actionName: `网关节点 ${token.nodeName} 成功完成入网认证，凭证立即销毁失效`,
      targetResource: `节点: ${token.nodeName} / 凭证: ${token.id}`,
      requestMethod: 'POST',
      requestUri: '/api/v1/enrollment/handshake',
      ip: assignedIp,
      location: token.networkName.includes('上海') ? '上海 · 浦东数据中心' : '北京 · 核心机房',
      device: `Edge-Gateway / ${hostname}`,
      status: 'SUCCESS',
      durationMs: 88,
      diffSnapshot: {
        before: { tokenStatus: 'PENDING', nodeStatus: 'pending_enrollment' },
        after: {
          tokenStatus: 'USED (一次性凭证已立即销毁)',
          nodeStatus: 'online',
          handshakeIp: assignedIp,
          virtualIp: updatedNode.virtualIp,
          allowedSubnets: token.allowedSubnets,
          permissionsEnforced: 'node:register_only (无控制面与业务数据越权)',
        },
      },
    });

    return {
      success: true,
      message: `网关节点 [${token.nodeName}] 入网握手成功！节点已激活上线 (在线 IP: ${assignedIp})，一次性注册凭证已按照安全策略立即销毁失效。`,
      node: updatedNode,
      token: { ...token },
    };
  },

  async deleteEnrollmentToken(id: string): Promise<boolean> {
    await delay(120);
    enrollmentTokensStore = enrollmentTokensStore.filter((t) => t.id !== id);
    return true;
  },
};


