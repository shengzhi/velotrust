import { jrpcCall } from './jrpc';

export type IPAMStatus = 'active' | 'disabled';

export interface IPAMPool {
  id: number;
  pool_name: string;
  cidr: string;
  type: 'Gateway' | 'User-Device' | 'Service-VIP';
  allocation_mode: 'static' | 'dynamic';
  idp_group?: string | null;
  client_group_id?: number | null;
  client_group_name?: string;
  gateway_binding?: string | null;
  color: string;
  description?: string | null;
  status: IPAMStatus;
  used: number;
  total: number;
  version_no: number;
}

export interface IPAMLease {
  id: number;
  pool_id: number;
  virtual_ip: string;
  node_type: 'Gateway' | 'Peer' | 'Server';
  target_name: string;
  endpoint: string;
  allocation_mode: string;
  idp_group?: string | null;
  wireguard_pubkey: string;
  status: 'Active' | 'Reserved' | 'Released';
  pool_name: string;
  allocated_at: string;
  released_at?: string | null;
}

export interface IPAMOverview {
  global_cidr: string;
  global_network_version: number;
  total_ips: number;
  used_ips: number;
  free_ips: number;
  ipam_pools: IPAMPool[];
  active_leases: IPAMLease[];
}

export interface PoolGroupOption {
  id: number;
  name: string;
  code: string;
  user_count: number;
}

export interface IPAMPoolInput {
  pool_name: string;
  cidr: string;
  pool_type: IPAMPool['type'];
  allocation_mode: IPAMPool['allocation_mode'];
  client_group_id: number | null;
  gateway_binding?: string;
  color: string;
  description?: string;
}

export interface IPAMConflict {
  isValid: boolean;
  hasConflict: boolean;
  message: string;
  conflictTarget?: string;
  isWithinOverlay?: boolean;
}

const listItems = <T,>(result: { items?: T[] | null } | null | undefined): T[] =>
  Array.isArray(result?.items) ? result.items : [];

export const ipamApi = {
  overview: () =>
    jrpcCall<Partial<IPAMOverview> | null>('IPAM_Overview', {}).then((result): IPAMOverview => ({
      global_cidr: result?.global_cidr ?? '',
      global_network_version: result?.global_network_version ?? 0,
      total_ips: result?.total_ips ?? 0,
      used_ips: result?.used_ips ?? 0,
      free_ips: result?.free_ips ?? 0,
      ipam_pools: Array.isArray(result?.ipam_pools) ? result.ipam_pools : [],
      active_leases: Array.isArray(result?.active_leases) ? result.active_leases : [],
    })),
  saveOverlayNetwork: (cidr: string, expectedVersion: number) =>
    jrpcCall<{ id: number; cidr: string; version_no: number }>('IPAM_SaveOverlayNetwork', { cidr, expected_version: expectedVersion }),
  listPools: () => jrpcCall<{ items?: IPAMPool[] | null } | null>('IPAM_ListPools', {}).then(listItems),
  createPool: (input: IPAMPoolInput) => jrpcCall<IPAMPool>('IPAM_CreatePool', input),
  updatePool: (pool: IPAMPool, input: IPAMPoolInput) =>
    jrpcCall<IPAMPool>('IPAM_UpdatePool', { pool_id: pool.id, expected_version: pool.version_no, ...input }),
  setPoolStatus: (pool: IPAMPool, status: IPAMStatus) =>
    jrpcCall<{ success: boolean }>('IPAM_SetPoolStatus', { pool_id: pool.id, status, expected_version: pool.version_no }),
  listLeases: () => jrpcCall<{ items?: IPAMLease[] | null } | null>('IPAM_ListLeases', {}).then(listItems),
  listEmployeeGroups: () => jrpcCall<{ items?: PoolGroupOption[] | null } | null>('IPAM_ListEmployeeGroups', {}).then(listItems),
  allocateAddress: (input: { pool_id: number; target_name: string; node_type: IPAMLease['node_type']; requested_ip?: string; idp_group?: string; wireguard_pubkey?: string; endpoint?: string }) =>
    jrpcCall<IPAMLease>('IPAM_AllocateAddress', input),
  releaseLease: (leaseId: number) => jrpcCall<{ success: boolean }>('IPAM_ReleaseLease', { lease_id: leaseId }),
  checkPrefix: (cidr: string) => jrpcCall<IPAMConflict>('IPAM_CheckPrefix', { cidr }),
};
