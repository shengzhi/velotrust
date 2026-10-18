import { jrpcCall } from './jrpc';

export interface GatewayEnrollmentTarget {
  id: number;
  gateway_id: string;
  gateway_name: string;
  gateway_type: string;
  network_name: string;
  allowed_subnets: string[];
  require_approval: boolean;
  status: string;
  registered_at?: string | null;
  created_by_name?: string;
  created_at: string;
  updated_at: string;
  version_no: number;
}

export type RegistrationCredentialStatus = 'pending' | 'used' | 'expired' | 'revoked';

export interface RegistrationCredential {
  id: number;
  gateway_id: string;
  gateway_name: string;
  gateway_type: string;
  network_name: string;
  allowed_subnets: string[];
  require_approval: boolean;
  status: RegistrationCredentialStatus;
  max_uses: number;
  used_count: number;
  expires_at: string;
  notes?: string | null;
  created_by_name: string;
  created_at: string;
  used_at?: string | null;
  used_ip?: string | null;
  revoked_at?: string | null;
  revocation_reason?: string | null;
}

export interface CredentialGrant {
  credential: RegistrationCredential;
  token: string;
}

export interface SelectableSiteNetwork {
  id: number;
  network_name: string;
  network_code: string;
  network_type: string;
  cidr: string;
  region?: string | null;
  environment?: string | null;
  description?: string | null;
  status: 'active' | 'disabled';
}

export interface CreateGatewayEnrollmentInput {
  gateway_name: string;
  gateway_type: string;
  network_name: string;
  allowed_network_ids: number[];
  require_approval: boolean;
  expires_in_hours: number;
  notes: string;
}

export const enrollmentApi = {
  listGateways: () =>
    jrpcCall<{ items: GatewayEnrollmentTarget[] }>('ENROLLMENT_ListGateways', {}).then((result) => result.items ?? []),

  listCredentials: () =>
    jrpcCall<{ items: RegistrationCredential[] }>('ENROLLMENT_ListCredentials', {}).then((result) => result.items ?? []),

  listSelectableSiteNetworks: () =>
    jrpcCall<{ items: SelectableSiteNetwork[] }>('ENROLLMENT_ListSelectableSiteNetworks', {}).then((result) => result.items ?? []),

  createGateway: (input: CreateGatewayEnrollmentInput) =>
    jrpcCall<CredentialGrant>('ENROLLMENT_CreateGateway', input),

  issueCredential: (gatewayRowID: number, expiresInHours: number, notes: string) =>
    jrpcCall<CredentialGrant>('ENROLLMENT_IssueCredential', {
      gateway_row_id: gatewayRowID,
      expires_in_hours: expiresInHours,
      notes,
    }),

  revokeCredential: (credentialID: number, reason: string) =>
    jrpcCall<{ success: boolean }>('ENROLLMENT_RevokeCredential', { credential_id: credentialID, reason }),

  extendCredential: (credentialID: number, additionalHours: number) =>
    jrpcCall<{ success: boolean }>('ENROLLMENT_ExtendCredential', {
      credential_id: credentialID,
      additional_hours: additionalHours,
    }),
};
