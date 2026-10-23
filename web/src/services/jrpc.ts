export const IAM_TOKEN_KEY = 'velotrust.iam.session';

interface RPCErrorPayload {
  code?: number;
  message?: string;
  data?: {
    code?: string;
    category?: string;
  };
}

interface RPCEnvelope<T> {
  result?: T;
  error?: RPCErrorPayload | null;
  code?: number;
  id?: string;
}

export class JRPCError extends Error {
  readonly rpcCode?: number;
  readonly businessCode?: string;

  constructor(message: string, rpcCode?: number, businessCode?: string) {
    super(message);
    this.name = 'JRPCError';
    this.rpcCode = rpcCode;
    this.businessCode = businessCode;
  }
}

const endpoint = import.meta.env.VITE_MANAGEMENT_JRPC_URL || '/jrpc';
let requestSequence = 0;

const businessMessages: Record<string, string> = {
  IAM_LOGIN_FAILED: '用户名或密码错误，或多因素验证码无效。',
  IAM_UNAUTHENTICATED: '登录状态已失效，请重新登录。',
  IAM_PERMISSION_DENIED: '当前账号没有执行此操作的权限。',
  IAM_INVALID_ARGUMENT: '提交的数据无效，请检查输入内容。',
  IAM_REAUTHENTICATION_REQUIRED: '验证失败，请确认当前密码和验证码。',
  IAM_TOTP_INVALID: '身份验证器验证码无效或已使用。',
  IAM_TOTP_ALREADY_ACTIVE: '此账号已启用 TOTP。',
  IAM_TOTP_NOT_ACTIVE: '此账号尚未启用 TOTP。',
  IAM_TOTP_SETUP_NOT_FOUND: 'TOTP 配置已过期，请重新开始绑定。',
  IAM_PASSKEY_CHALLENGE_INVALID: '通行密钥验证已过期，请重新操作。',
  IAM_PASSKEY_INVALID: '通行密钥验证失败。',
  IAM_WEBAUTHN_ORIGIN_INVALID: '当前访问域名不在 Management 的 WebAuthn 允许列表中。',
  IAM_USER_STATUS_DENIED: '不能禁用当前账号或最后一个有效平台管理员。',
  IAM_ROLE_GRANT_DENIED: '不能向当前登录账号授予或撤销角色。',
  ENROLLMENT_INVALID_ARGUMENT: '注册凭证信息不完整或格式不正确，请检查输入内容。',
  ZTNA_NODE_INVALID_ARGUMENT: '节点参数无效，请检查提交内容。',
  ZTNA_NODE_NOT_FOUND: '虚拟节点不存在或已被移除。',
  ZTNA_NODE_VERSION_CONFLICT: '节点信息已被其他操作修改，请刷新后重试。',
  ZTNA_NODE_TRANSITION_DENIED: '当前节点状态不允许执行此状态变更。',
  ZTNA_NODE_ACTIVATION_REQUIRES_REGISTRATION: '节点完成注册并提交公钥后才能启用。',
  ZTNA_POLICY_INVALID_ARGUMENT: '访问策略或 ACL 规则无效，请检查网段、协议、端口和 Gateway。',
  ZTNA_POLICY_NOT_FOUND: '访问策略不存在或已被删除。',
  ZTNA_POLICY_VERSION_CONFLICT: '访问策略已被其他管理员修改，请刷新后重试。',
};

function errorMessage(error: RPCErrorPayload | null | undefined): string {
  if (!error) return 'Management 请求失败。';
  const businessCode = error.data?.code;
  if (businessCode && businessMessages[businessCode]) return businessMessages[businessCode];
  if (error.code === -32001) return '登录状态无效或已过期，请重新登录。';
  if (error.code === -32003) return '当前账号没有执行此操作的权限。';
  if (error.code === -32009) return '数据冲突，可能已被其他管理员修改。';
  if (error.code === -32602) return '请求参数无效，请检查输入内容。';
  if (error.message === 'Internal error' || error.code === -32603) return 'Management 服务暂时无法处理请求。';
  return error.message || 'Management 请求失败。';
}

export async function jrpcCall<T>(method: string, params: object = {}, token?: string): Promise<T> {
  const headers = new Headers({
    Accept: 'application/json',
    'Content-Type': 'application/json',
  });
  const bearerToken = token ?? sessionStorage.getItem(IAM_TOKEN_KEY);
  if (bearerToken) headers.set('Authorization', `Bearer ${bearerToken}`);

  let response: Response;
  try {
    response = await fetch(endpoint, {
      method: 'POST',
      headers,
      credentials: 'same-origin',
      body: JSON.stringify({
        jsonrpc: '2.0',
        id: `web-${Date.now()}-${++requestSequence}`,
        method,
        params,
      }),
    });
  } catch {
    throw new JRPCError('无法连接 Management。请确认服务已启动且 Web 的 JRPC 代理地址正确。');
  }

  let envelope: RPCEnvelope<T>;
  try {
    envelope = (await response.json()) as RPCEnvelope<T>;
  } catch {
    throw new JRPCError(`Management 返回了无法识别的响应（HTTP ${response.status}）。`);
  }

  if (!response.ok) {
    throw new JRPCError(`Management 请求失败（HTTP ${response.status}）。`);
  }
  if (envelope.error || (envelope.code !== undefined && envelope.code !== 200)) {
    const err = envelope.error;
    if (err?.code === -32001) {
      window.dispatchEvent(new Event('velotrust:iam-unauthorized'));
    }
    throw new JRPCError(errorMessage(err), err?.code, err?.data?.code);
  }
  if (envelope.result === undefined) {
    throw new JRPCError('Management 响应缺少 result。');
  }
  return envelope.result;
}
