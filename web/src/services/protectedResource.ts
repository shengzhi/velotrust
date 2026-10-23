import { jrpcCall } from './jrpc';
import type { ProtectedResource, ResourceCategory, ResourceProtocol } from '../types';

export interface ProtectedResourceRecord {
  id: number;
  version_no: number;
  resource_name: string;
  resource_code: string;
  resource_type: string;
  network_id: number;
  network_name: string;
  network_cidr: string;
  gateway_node_id: number;
  gateway_name: string;
  host?: string | null;
  address: string;
  port_start: number | null;
  port_end: number | null;
  protocol: string;
  access_mode: string;
  category?: string | null;
  tags: string[];
  description?: string | null;
  status: string;
  policy_count: number;
}

export interface ProtectedResourceInput {
  id?: number;
  version_no?: number;
  resource_name: string;
  resource_code: string;
  resource_type: string;
  network_id: number;
  host?: string;
  address: string;
  port_start: number;
  port_end: number;
  protocol: 'tcp' | 'udp';
  access_mode: string;
  category?: string;
  tags: string[];
  description?: string | null;
  status: 'active' | 'disabled';
}

function category(value: string): ResourceCategory {
  const allowed: ResourceCategory[] = ['web_portal', 'database', 'server_ssh', 'rdp_desktop', 'api_gateway', 'cloud_console'];
  return allowed.includes(value as ResourceCategory) ? value as ResourceCategory : 'web_portal';
}

function displayProtocol(protocol: string, port: number, portEnd: number): ResourceProtocol {
  if (protocol === 'udp') return 'UDP';
  if (port <= 0 || portEnd <= 0 || port !== portEnd) return 'TCP';
  if (port === 22) return 'SSH';
  if (port === 3389) return 'RDP';
  if (port === 3306) return 'MySQL';
  if (port === 5432) return 'PostgreSQL';
  if (port === 80) return 'HTTP';
  if (port === 443) return 'HTTPS';
  return 'TCP';
}

function toView(record: ProtectedResourceRecord): ProtectedResource & { versionNo: number } {
  const port = record.port_start || 0;
  const portEnd = record.port_end || 0;
  const host = record.host || record.address || '';
  const portMode = port === 0 && portEnd === 0 ? 'unconfigured'
    : port === 1 && portEnd === 65535 ? 'any'
      : port === portEnd ? 'single' : 'range';
  return {
    id: String(record.id), versionNo: record.version_no, name: record.resource_name, code: record.resource_code,
    category: category(record.category || record.resource_type), protocol: displayProtocol(record.protocol, port, portEnd),
    host, port, portEnd, portMode,
    siteId: String(record.network_id), siteName: record.network_name,
    gatewayNodeId: String(record.gateway_node_id || ''), gatewayNodeName: record.gateway_name || '未指定 Gateway',
    sensitivity: 'Internal', department: '', owner: '', status: record.status === 'active' ? 'healthy' : 'unreachable',
    latencyMs: 0, healthCheckType: 'TCP_SYN', associatedPoliciesCount: record.policy_count,
    tags: record.tags || [], description: record.description || '', createdAt: '',
  };
}

function transport(protocol: ResourceProtocol): 'tcp' | 'udp' { return protocol === 'UDP' ? 'udp' : 'tcp'; }

export const protectedResourceApi = {
  list: () => jrpcCall<{ items?: ProtectedResourceRecord[] | null }>('RESOURCE_ListResources', {}).then((result) =>
    Array.isArray(result?.items) ? result.items.map(toView) : [],
  ),
  save: async (resource: Partial<ProtectedResource> & { name: string; host: string; siteId: string }): Promise<ProtectedResource> => {
    const row = await jrpcCall<ProtectedResourceRecord>('RESOURCE_SaveResource', {
      id: resource.id ? Number(resource.id) : 0,
      version_no: (resource as ProtectedResource & { versionNo?: number }).versionNo || 0,
      resource_name: resource.name,
      resource_code: resource.code,
      resource_type: resource.category || 'web_portal',
      network_id: Number(resource.siteId),
      host: resource.host,
      address: resource.host,
      port_start: resource.portMode === 'any' ? 1 : resource.port || 0,
      port_end: resource.portMode === 'any' ? 65535 : resource.portMode === 'range' ? resource.portEnd || 0 : resource.port || 0,
      protocol: transport(resource.protocol || 'TCP'),
      access_mode: 'network',
      category: resource.category || 'web_portal',
      tags: resource.tags || [],
      description: resource.description || null,
      status: 'active',
    } satisfies ProtectedResourceInput);
    return toView(row);
  },
  delete: (resource: ProtectedResource) => jrpcCall<{ success: boolean }>('RESOURCE_DeleteResource', {
    id: Number(resource.id),
    expected_version: (resource as ProtectedResource & { versionNo?: number }).versionNo || 0,
  }),
};
