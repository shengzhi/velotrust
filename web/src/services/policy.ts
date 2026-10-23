import { jrpcCall } from './jrpc';

export interface AccessPolicyRule {
  id: string;
  priority: number;
  action: 'allow' | 'deny';
}

export interface AccessPolicy {
  id: number;
  version_no: number;
  policy_name: string;
  policy_code: string;
  description?: string | null;
  priority: number;
  status: 'draft' | 'active' | 'disabled';
  rules: AccessPolicyRule[];
  subjects?: { subject_type: string; subject_id: number }[];
  resource_ids?: number[];
}

export interface PolicyGatewayOption {
  id: number;
  name: string;
  node_code: string;
  status: string;
}

export interface PolicyGroupOption {
  id: number;
  name: string;
  code: string;
  status: string;
  member_count: number;
}

export interface PolicyResourceOption {
  id: number;
  version_no: number;
  name: string;
  code: string;
  address?: string | null;
  network_id?: number | null;
  network_name?: string | null;
  network_cidr?: string | null;
  gateway_node_id?: number | null;
  gateway_name?: string | null;
  protocol?: string | null;
  port_start?: number | null;
  port_end?: number | null;
  status: string;
  description?: string | null;
  category?: string | null;
}

export const policyApi = {
  list: () =>
    jrpcCall<{ items?: AccessPolicy[] | null }>('POLICY_ListPolicies', {}).then((result) =>
      Array.isArray(result?.items) ? result.items : [],
    ),
  listGateways: () =>
    jrpcCall<{ items?: PolicyGatewayOption[] | null }>('POLICY_ListGateways', {}).then((result) =>
      Array.isArray(result?.items) ? result.items : [],
    ),
  listGroups: () =>
    jrpcCall<{ items?: PolicyGroupOption[] | null }>('POLICY_ListGroups', {}).then((result) =>
      Array.isArray(result?.items) ? result.items : [],
    ),
  listResources: () =>
    jrpcCall<{ items?: PolicyResourceOption[] | null }>('POLICY_ListResources', {}).then((result) =>
      Array.isArray(result?.items) ? result.items : [],
    ),
  save: (policy: Partial<AccessPolicy>) => jrpcCall<AccessPolicy>('POLICY_SavePolicy', policy),
  setStatus: (policy: AccessPolicy, status: AccessPolicy['status']) =>
    jrpcCall<{ success: boolean }>('POLICY_SetPolicyStatus', {
      id: policy.id,
      expected_version: policy.version_no,
      status,
    }),
  delete: (policy: AccessPolicy) =>
    jrpcCall<{ success: boolean }>('POLICY_DeletePolicy', {
      id: policy.id,
      expected_version: policy.version_no,
    }),
};
