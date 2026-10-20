import { jrpcCall } from './jrpc';

export type VirtualNodeType = 'client_agent' | 'gateway' | 'relay' | 'connector';
export type VirtualNodeStatus = 'pending' | 'active' | 'disabled' | 'quarantined' | 'revoked';
export type VirtualNodeHealth = 'unknown' | 'healthy' | 'degraded' | 'unhealthy';

export interface VirtualNodeNetwork {
  id: number;
  network_name: string;
  network_code: string;
  network_type: string;
  cidr: string;
  status: 'active' | 'disabled';
}

export interface VirtualNodePeer {
  id: number;
  tunnel_id: number;
  tunnel_code: string;
  tunnel_ip: string;
  allowed_ips: string[];
  endpoint?: string | null;
  status: string;
  last_handshake_at?: string | null;
}

export interface VirtualNode {
  id: number;
  version_no: number;
  node_name: string;
  node_code: string;
  node_type: VirtualNodeType;
  node_status: VirtualNodeStatus;
  agent_version?: string | null;
  control_endpoint?: string | null;
  public_endpoint?: string | null;
  private_endpoint?: string | null;
  public_key?: string | null;
  last_heartbeat_at?: string | null;
  last_config_hash?: string | null;
  actual_config_hash?: string | null;
  capabilities: Record<string, unknown>;
  health_status: VirtualNodeHealth;
  gateway_type?: string | null;
  gateway_network_name?: string | null;
  require_approval: boolean;
  registered_at?: string | null;
  created_at: string;
  networks: VirtualNodeNetwork[];
  peers: VirtualNodePeer[];
}

export const virtualNodeApi = {
  list: () =>
    jrpcCall<{ items?: VirtualNode[] | null }>('NODE_ListNodes', {}).then((result) =>
      Array.isArray(result?.items) ? result.items : [],
    ),
  get: (id: number) => jrpcCall<VirtualNode>('NODE_GetNode', { id }),
  setStatus: (node: VirtualNode, nodeStatus: VirtualNodeStatus) =>
    jrpcCall<{ success: boolean }>('NODE_SetNodeStatus', {
      id: node.id,
      node_status: nodeStatus,
      expected_version: node.version_no,
    }),
};
