import { jrpcCall } from './jrpc';
import { Site } from '../types';

interface SiteNATMapping {
  id: number;
  version_no: number;
  enabled: boolean;
  virtual_cidr?: string | null;
  nat_type: '1:1_Bidirectional' | 'SNAT_Masquerade' | 'DNAT_VirtualIP';
  description?: string | null;
}

interface SiteRecord {
  id: number;
  site_code: string;
  name: string;
  gateway_node_id?: number | null;
  gateway_name: string;
  status: 'active' | 'disabled';
  network_id: number;
  network_code: string;
  local_subnet: string;
  node_count: number;
  nat_mapping?: SiteNATMapping | null;
  created_at: string;
  version_no: number;
}

export interface SiteGatewayOption {
  id: number;
  gateway_id: string;
  gateway_name: string;
  status: string;
}

export interface SaveSiteInput {
  id?: number;
  expected_version?: number;
  name: string;
  local_subnet: string;
  gateway_node_id?: number | null;
  status: 'active' | 'disabled';
}

export interface SaveSiteNATMappingInput {
  id: number;
  expected_version?: number;
  network_id: number;
  nat_type: '1:1_Bidirectional' | 'SNAT_Masquerade' | 'DNAT_VirtualIP';
  virtual_cidr?: string | null;
  enabled: boolean;
  description?: string;
}

const toSite = (item: SiteRecord): Site => ({
  id: String(item.id),
  networkId: item.network_id,
  versionNo: item.version_no,
  name: item.name,
  localSubnet: item.local_subnet,
  gatewayNodeId: item.gateway_node_id ? String(item.gateway_node_id) : '',
  gatewayNodeName: item.gateway_name || '未指定网关',
  nodeCount: item.node_count ?? 0,
  status: item.status === 'disabled' ? 'inactive' : 'active',
  createdAt: item.created_at,
  natMapping: item.nat_mapping
    ? {
        id: item.nat_mapping.id,
        version_no: item.nat_mapping.version_no,
        network_id: item.network_id,
        enabled: item.nat_mapping.enabled,
        virtualCidr: item.nat_mapping.virtual_cidr ?? '',
        natType: item.nat_mapping.nat_type,
        description: item.nat_mapping.description ?? '',
      }
    : undefined,
});

export const siteApi = {
  listSites: () =>
    jrpcCall<{ items?: SiteRecord[] | null }>('SITE_ListSites', {}).then((result) =>
      Array.isArray(result?.items) ? result.items.map(toSite) : [],
    ),
  listGateways: () =>
    jrpcCall<{ items?: SiteGatewayOption[] | null }>('SITE_ListGateways', {}).then((result) => result.items ?? []),
  saveSite: async (input: SaveSiteInput): Promise<Site> => {
    const saved = await jrpcCall<SiteRecord>('SITE_SaveSite', input);
    return toSite(saved);
  },
  saveNATMapping: (mapping: SaveSiteNATMappingInput) =>
    jrpcCall<{ id: number; version_no: number }>('SITE_SaveNATMapping', mapping),
  deleteSite: (site: Site) =>
    jrpcCall<{ success: boolean }>('SITE_DeleteSite', {
      id: Number(site.id),
      expected_version: site.versionNo,
    }),
};
