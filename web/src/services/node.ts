import { jrpcCall } from './jrpc';

export type VirtualNodeType = 'client_agent' | 'gateway' | 'relay' | 'connector';
export type VirtualNodeStatus = 'pending' | 'active' | 'disabled' | 'quarantined' | 'revoked';
export type VirtualNodeHealth = 'unknown' | 'healthy' | 'degraded' | 'unhealthy';
export type VirtualNodeConnectionStatus = 'online' | 'offline' | 'unknown';

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
  connection_status: VirtualNodeConnectionStatus;
  agent_version?: string | null;
  control_endpoint?: string | null;
  public_endpoint?: string | null;
  private_endpoint?: string | null;
  ipam_address?: string | null;
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
  device_id?: number | null;
  device_name?: string | null;
  device_type?: string | null;
  device_platform?: string | null;
  device_status?: 'pending' | 'active' | 'disabled' | 'quarantined' | 'revoked' | null;
  owner_user_id?: number | null;
  owner_username?: string | null;
}

export interface AccessChangeStatus {
  event_id: number;
  subject_type: 'user' | 'device' | 'node';
  subject_id: number;
  status: 'pending' | 'completed';
  target_count: number;
  acknowledged_count: number;
  last_error?: string | null;
  created_at: string;
  last_acknowledged_at?: string | null;
}

export interface GatewayTunnelOption {
  id: number;
  code: string;
  network_name?: string;
}

export const virtualNodeApi = {
  list: () =>
    jrpcCall<{ items?: VirtualNode[] | null }>('NODE_ListNodes', {}).then((result) =>
      Array.isArray(result?.items) ? result.items : [],
    ),
  get: (id: number) => jrpcCall<VirtualNode>('NODE_GetNode', { id }),
  getAccessChangeStatus: (subjectType: AccessChangeStatus['subject_type'], subjectID: number) =>
    jrpcCall<AccessChangeStatus | null>('NODE_GetAccessChangeStatus', {
      subject_type: subjectType,
      subject_id: subjectID,
    }),
  listGatewayTunnelOptions: () =>
    jrpcCall<{ items?: GatewayTunnelOption[] | null }>('NODE_ListGatewayTunnelOptions', {}).then((result) =>
      Array.isArray(result?.items) ? result.items : [],
    ),
  createLogicalTunnel: (code: string, description: string) =>
    jrpcCall<GatewayTunnelOption>('NODE_CreateLogicalTunnel', { tunnel_code: code, description }),
  assignGatewayTunnel: (node: VirtualNode, tunnelID: number) =>
    jrpcCall<{ success: boolean }>('NODE_AssignGatewayTunnel', {
      node_id: node.id,
      tunnel_id: tunnelID,
      expected_version: node.version_no,
    }),
  setStatus: (node: VirtualNode, nodeStatus: VirtualNodeStatus) =>
    jrpcCall<{ success: boolean }>('NODE_SetNodeStatus', {
      id: node.id,
      node_status: nodeStatus,
      expected_version: node.version_no,
    }),
  setPublicEndpoint: (node: VirtualNode, publicEndpoint: string) =>
    jrpcCall<{ success: boolean }>('NODE_SetNodePublicEndpoint', {
      id: node.id,
      public_endpoint: publicEndpoint,
      expected_version: node.version_no,
    }),
};
