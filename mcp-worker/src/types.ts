import type { OAuthHelpers } from "@cloudflare/workers-oauth-provider";

export interface Env {
  OAUTH_KV: KVNamespace;
  GITHUB_CLIENT_ID: string;
  GITHUB_CLIENT_SECRET: string;
  OAUTH_STATE_SECRET: string;
  TURSO_URL: string;
  TURSO_TOKEN: string;
}

// OAuthProvider injects helpers into the default handler's env.
export interface AppEnv extends Env {
  OAUTH_PROVIDER: OAuthHelpers;
}

// Stored in the OAuth grant and surfaced on ExecutionContext.props for API requests.
export interface Props extends Record<string, unknown> {
  login: string;
}
