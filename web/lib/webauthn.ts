// Convert between WebAuthn ArrayBuffers and the base64url JSON that go-webauthn emits
// (challenge, user.id, credential id / rawId / attestation fields). Browser APIs want
// ArrayBuffers; POST /auth/passkey/*/finish wants the JSON form.

import { parseControlPlaneError } from "./agent-root";
import type { Passkey, Server } from "./types";

export type { Passkey };

type BeginResponse = {
  publicKey: Record<string, unknown>;
  challenge_id: string;
};

type DescriptorJSON = {
  type: PublicKeyCredentialType;
  id: string;
  transports?: AuthenticatorTransport[];
};

function bytesOf(buf: BufferSource): Uint8Array {
  if (buf instanceof ArrayBuffer) return new Uint8Array(buf);
  if (ArrayBuffer.isView(buf)) {
    return new Uint8Array(buf.buffer, buf.byteOffset, buf.byteLength);
  }
  throw new TypeError("expected BufferSource");
}

export function bufferToBase64url(buf: BufferSource): string {
  const bytes = bytesOf(buf);
  let bin = "";
  for (const b of bytes) bin += String.fromCharCode(b);
  return btoa(bin).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

export function base64urlToBuffer(value: string): ArrayBuffer {
  const padded = value.replace(/-/g, "+").replace(/_/g, "/").padEnd(Math.ceil(value.length / 4) * 4, "=");
  const bin = atob(padded);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out.buffer;
}

function decodeDescriptor(desc: DescriptorJSON): PublicKeyCredentialDescriptor {
  return {
    type: desc.type,
    id: base64urlToBuffer(desc.id),
    transports: desc.transports,
  };
}

export function toCreationOptions(publicKey: Record<string, unknown>): CredentialCreationOptions {
  const user = publicKey.user as { id: string; name: string; displayName: string };
  const exclude = publicKey.excludeCredentials as DescriptorJSON[] | undefined;
  return {
    publicKey: {
      ...(publicKey as unknown as PublicKeyCredentialCreationOptions),
      challenge: base64urlToBuffer(publicKey.challenge as string),
      user: { ...user, id: base64urlToBuffer(user.id) },
      ...(exclude ? { excludeCredentials: exclude.map(decodeDescriptor) } : {}),
    },
  };
}

export type RequestExtras = {
  mediation?: CredentialMediationRequirement;
  signal?: AbortSignal;
};

export function toRequestOptions(
  publicKey: Record<string, unknown>,
  extras?: RequestExtras,
): CredentialRequestOptions {
  const allow = publicKey.allowCredentials as DescriptorJSON[] | undefined;
  return {
    publicKey: {
      ...(publicKey as unknown as PublicKeyCredentialRequestOptions),
      challenge: base64urlToBuffer(publicKey.challenge as string),
      ...(allow ? { allowCredentials: allow.map(decodeDescriptor) } : {}),
    },
    ...(extras?.mediation ? { mediation: extras.mediation } : {}),
    ...(extras?.signal ? { signal: extras.signal } : {}),
  };
}

export function normalizePasskeyName(name?: string | null): string {
  const trimmed = name?.trim() ?? "";
  return trimmed || "Passkey";
}

type ConditionalMediationAPI = {
  isConditionalMediationAvailable?: () => Promise<boolean>;
};

export async function supportsConditionalMediation(): Promise<boolean> {
  const api = (globalThis as { PublicKeyCredential?: ConditionalMediationAPI }).PublicKeyCredential;
  if (!api || typeof api.isConditionalMediationAvailable !== "function") return false;
  try {
    return await api.isConditionalMediationAvailable();
  } catch {
    return false;
  }
}

export function credentialToJSON(cred: PublicKeyCredential): Record<string, unknown> {
  const withJSON = cred as PublicKeyCredential & { toJSON?: () => Record<string, unknown> };
  if (typeof withJSON.toJSON === "function") return withJSON.toJSON();

  const response = cred.response as AuthenticatorAttestationResponse & AuthenticatorAssertionResponse;
  const json: Record<string, unknown> = {
    id: cred.id,
    rawId: bufferToBase64url(cred.rawId),
    type: cred.type,
    clientExtensionResults: cred.getClientExtensionResults?.() ?? {},
  };
  if (cred.authenticatorAttachment) {
    json.authenticatorAttachment = cred.authenticatorAttachment;
  }
  if ("attestationObject" in cred.response) {
    json.response = {
      clientDataJSON: bufferToBase64url(response.clientDataJSON),
      attestationObject: bufferToBase64url(response.attestationObject),
      transports: typeof response.getTransports === "function" ? response.getTransports() : [],
    };
  } else {
    json.response = {
      clientDataJSON: bufferToBase64url(response.clientDataJSON),
      authenticatorData: bufferToBase64url(response.authenticatorData),
      signature: bufferToBase64url(response.signature),
      userHandle: response.userHandle ? bufferToBase64url(response.userHandle) : null,
    };
  }
  return json;
}

async function readError(res: Response, fallback: string): Promise<string> {
  return parseControlPlaneError(res.status, await res.text().catch(() => ""), fallback).message;
}

async function postJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: "POST",
    credentials: "include",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    throw parseControlPlaneError(res.status, await res.text().catch(() => ""), "request failed");
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export async function registerPasskey(name?: string | null): Promise<Passkey> {
  const begin = await postJSON<BeginResponse>("/api/auth/passkey/register/begin", {});
  const cred = await navigator.credentials.create(toCreationOptions(begin.publicKey));
  if (!cred || cred.type !== "public-key") throw new Error("Passkey creation was cancelled");
  return postJSON<Passkey>("/api/auth/passkey/register/finish", {
    challenge_id: begin.challenge_id,
    name: normalizePasskeyName(name),
    credential: credentialToJSON(cred as PublicKeyCredential),
  });
}

export async function loginWithPasskey(extras?: RequestExtras): Promise<void> {
  const begin = await postJSON<BeginResponse>("/api/auth/passkey/login/begin", {});
  const cred = await navigator.credentials.get(toRequestOptions(begin.publicKey, extras));
  if (!cred || cred.type !== "public-key") throw new Error("Passkey sign-in was cancelled");
  await postJSON("/api/auth/passkey/login/finish", {
    challenge_id: begin.challenge_id,
    credential: credentialToJSON(cred as PublicKeyCredential),
  });
}

export async function listPasskeys(): Promise<Passkey[]> {
  const res = await fetch("/api/auth/passkeys", { credentials: "include" });
  if (!res.ok) throw new Error(await readError(res, "Could not load passkeys"));
  const body = (await res.json()) as { items?: Passkey[] };
  return body.items ?? [];
}

export async function deletePasskey(id: string): Promise<void> {
  const res = await fetch(`/api/auth/passkeys/${encodeURIComponent(id)}`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!res.ok && res.status !== 204) throw new Error(await readError(res, "Could not delete passkey"));
}

export async function beginStepUp(): Promise<BeginResponse> {
  return postJSON<BeginResponse>("/api/auth/passkey/step-up/begin", {});
}

export async function stepUpWithPasskey(): Promise<{
  challenge_id: string;
  credential: Record<string, unknown>;
}> {
  const begin = await beginStepUp();
  const cred = await navigator.credentials.get(toRequestOptions(begin.publicKey));
  if (!cred || cred.type !== "public-key") throw new Error("Passkey verification was cancelled");
  return {
    challenge_id: begin.challenge_id,
    credential: credentialToJSON(cred as PublicKeyCredential),
  };
}

export async function setAgentRoot(serverId: string, enabled: boolean): Promise<Server> {
  const step = await stepUpWithPasskey();
  return postJSON<Server>(`/api/servers/${encodeURIComponent(serverId)}/agent-root`, {
    enabled,
    challenge_id: step.challenge_id,
    credential: step.credential,
  });
}
