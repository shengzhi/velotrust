type JSONOptions = Record<string, any>;

function decodeBase64URL(value: string): ArrayBuffer {
  const normalized = value.replace(/-/g, '+').replace(/_/g, '/');
  const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, '=');
  const binary = atob(padded);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index);
  return bytes.buffer;
}

function encodeBase64URL(value: ArrayBuffer): string {
  const bytes = new Uint8Array(value);
  let binary = '';
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '');
}

function publicKeyOptions<T extends JSONOptions>(envelope: T): JSONOptions {
  const options = envelope.publicKey ?? envelope;
  if (!options || typeof options.challenge !== 'string') {
    throw new Error('Management 返回的 WebAuthn challenge 格式无效。');
  }
  return options;
}

function assertWebAuthnAvailable(): void {
  if (!window.isSecureContext || !window.PublicKeyCredential || !navigator.credentials) {
    throw new Error('当前浏览器或访问地址不支持通行密钥；请使用 HTTPS 或 localhost，并启用 WebAuthn。');
  }
}

function serializeCredential(credential: PublicKeyCredential): object {
  const common = {
    id: credential.id,
    rawId: encodeBase64URL(credential.rawId),
    type: credential.type,
    authenticatorAttachment: credential.authenticatorAttachment,
    clientExtensionResults: credential.getClientExtensionResults(),
  };

  if (credential.response instanceof AuthenticatorAttestationResponse) {
    return {
      ...common,
      response: {
        clientDataJSON: encodeBase64URL(credential.response.clientDataJSON),
        attestationObject: encodeBase64URL(credential.response.attestationObject),
        transports: credential.response.getTransports?.() ?? [],
      },
    };
  }

  if (credential.response instanceof AuthenticatorAssertionResponse) {
    return {
      ...common,
      response: {
        clientDataJSON: encodeBase64URL(credential.response.clientDataJSON),
        authenticatorData: encodeBase64URL(credential.response.authenticatorData),
        signature: encodeBase64URL(credential.response.signature),
        ...(credential.response.userHandle
          ? { userHandle: encodeBase64URL(credential.response.userHandle) }
          : {}),
      },
    };
  }

  throw new Error('浏览器返回了无法识别的 WebAuthn 凭证。');
}

export async function createPasskey(optionsEnvelope: JSONOptions): Promise<object> {
  assertWebAuthnAvailable();
  const options = publicKeyOptions(optionsEnvelope);
  const publicKey = {
    ...options,
    challenge: decodeBase64URL(options.challenge),
    user: {
      ...options.user,
      id: decodeBase64URL(options.user?.id),
    },
    excludeCredentials: (options.excludeCredentials ?? []).map((credential: JSONOptions) => ({
      ...credential,
      id: decodeBase64URL(credential.id),
    })),
  } as PublicKeyCredentialCreationOptions;
  const credential = await navigator.credentials.create({ publicKey });
  if (!(credential instanceof PublicKeyCredential)) throw new Error('用户取消了通行密钥创建。');
  return serializeCredential(credential);
}

export async function getPasskey(optionsEnvelope: JSONOptions): Promise<object> {
  assertWebAuthnAvailable();
  const options = publicKeyOptions(optionsEnvelope);
  const publicKey = {
    ...options,
    challenge: decodeBase64URL(options.challenge),
    allowCredentials: (options.allowCredentials ?? []).map((credential: JSONOptions) => ({
      ...credential,
      id: decodeBase64URL(credential.id),
    })),
  } as PublicKeyCredentialRequestOptions;
  const credential = await navigator.credentials.get({ publicKey });
  if (!(credential instanceof PublicKeyCredential)) throw new Error('用户取消了通行密钥验证。');
  return serializeCredential(credential);
}
