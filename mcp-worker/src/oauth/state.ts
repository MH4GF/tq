import type { AuthRequest } from "@cloudflare/workers-oauth-provider";

function bytesToBase64url(bytes: Uint8Array): string {
  let binary = "";
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function base64urlToBytes(value: string): Uint8Array {
  const padded = value
    .replace(/-/g, "+")
    .replace(/_/g, "/")
    .padEnd(Math.ceil(value.length / 4) * 4, "=");
  const binary = atob(padded);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

async function importKey(secret: string): Promise<CryptoKey> {
  if (!secret) {
    throw new Error("OAUTH_STATE_SECRET is not set");
  }
  return crypto.subtle.importKey(
    "raw",
    new TextEncoder().encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign", "verify"],
  );
}

export async function signState(
  authRequest: AuthRequest,
  secret: string,
): Promise<string> {
  const payload = bytesToBase64url(
    new TextEncoder().encode(JSON.stringify(authRequest)),
  );
  const key = await importKey(secret);
  const signature = await crypto.subtle.sign(
    "HMAC",
    key,
    new TextEncoder().encode(payload),
  );
  return `${payload}.${bytesToBase64url(new Uint8Array(signature))}`;
}

export async function verifyState(
  state: string,
  secret: string,
): Promise<AuthRequest> {
  const dot = state.indexOf(".");
  if (dot === -1) {
    throw new Error("Malformed state");
  }
  const payload = state.slice(0, dot);
  const signature = base64urlToBytes(state.slice(dot + 1));
  const key = await importKey(secret);
  const valid = await crypto.subtle.verify(
    "HMAC",
    key,
    signature,
    new TextEncoder().encode(payload),
  );
  if (!valid) {
    throw new Error("Invalid state signature");
  }
  return JSON.parse(
    new TextDecoder().decode(base64urlToBytes(payload)),
  ) as AuthRequest;
}
