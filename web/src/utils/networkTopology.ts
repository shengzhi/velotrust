import { Node, Site } from '../types';

export interface BehindSubnetInfo {
  id: string;
  cidr: string;
  name: string;
  type: 'local' | 'nat_mapped' | 'advertised';
  routingMode: string;
  status: 'active' | 'standby';
  siteName: string;
  cloudProvider?: string;
  ipCount?: number;
  description?: string;
}

/**
 * Extracts all backend networks and advertised subnets behind a gateway node
 */
export function getBehindSubnetsForNode(node: Node, sites: Site[] = []): BehindSubnetInfo[] {
  const result: BehindSubnetInfo[] = [];
  const site = sites.find((s) => s.id === node.siteId || s.gatewayNodeId === node.id);

  // 1. If NAT mapping is enabled on the site, add the NAT mapped virtual CIDR
  if (site?.natMapping?.enabled && site.natMapping.virtualCidr) {
    result.push({
      id: `${node.id}-nat-${site.natMapping.virtualCidr}`,
      cidr: site.natMapping.virtualCidr,
      name: `${site.name} (跨云 1:1 NAT 影子网段)`,
      type: 'nat_mapped',
      routingMode:
        site.natMapping.natType === '1:1_Bidirectional'
          ? '双向 1:1 NAT 映射'
          : 'SNAT 地址伪装',
      status: 'active',
      siteName: site.name,
      cloudProvider: site.cloudProvider,
      ipCount: 254,
      description: site.natMapping.description || '跨云重叠网段虚拟映射，确保 Mesh 全网无冲突直达',
    });
  }

  // 2. If site has localSubnet and it's not already added
  if (site?.localSubnet && !result.some((r) => r.cidr === site.localSubnet)) {
    result.push({
      id: `${node.id}-local-${site.localSubnet}`,
      cidr: site.localSubnet,
      name: `${site.name} 本地原生 VPC/私网`,
      type: 'local',
      routingMode: '内核直接转发 (Kernel Direct Forwarding)',
      status: 'active',
      siteName: site.name,
      cloudProvider: site.cloudProvider,
      ipCount: 254,
      description: '站点机房内部实际分配的本地局域网 / VPC 原生网段',
    });
  }

  // 3. node.allowedSubnets
  if (node.allowedSubnets && node.allowedSubnets.length > 0) {
    node.allowedSubnets.forEach((cidr, idx) => {
      if (!result.some((r) => r.cidr === cidr)) {
        const is16 = cidr.includes('/16');
        const is20 = cidr.includes('/20');
        const ipCount = is16 ? 65534 : is20 ? 4094 : 254;

        result.push({
          id: `${node.id}-adv-${idx}-${cidr}`,
          cidr,
          name: node.networkName ? `${node.networkName} 通告网段` : `后端业务网段 (${cidr})`,
          type: 'advertised',
          routingMode: 'WireGuard AllowedIPs 路由通告',
          status: 'active',
          siteName: node.siteName,
          cloudProvider: site?.cloudProvider,
          ipCount,
          description: `网关通过 WireGuard AllowedIPs 实时向全网 Mesh 广播的可达子网`,
        });
      }
    });
  }

  // Fallback if none defined: generate appropriate network segment based on node
  if (result.length === 0) {
    const fallbackCidr = node.virtualIp.startsWith('10.0.0.')
      ? `192.168.${parseInt(node.virtualIp.split('.')[3] || '1', 10) * 10}.0/24`
      : '10.200.0.0/24';
    result.push({
      id: `${node.id}-fallback`,
      cidr: fallbackCidr,
      name: `${node.siteName} 默认后端网段`,
      type: 'local',
      routingMode: '内核直接转发 (Kernel Direct)',
      status: 'active',
      siteName: node.siteName,
      cloudProvider: site?.cloudProvider,
      ipCount: 254,
      description: '站点默认局域网段',
    });
  }

  return result;
}
