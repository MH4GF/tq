const GITHUB_AUTHORIZE_URL = "https://github.com/login/oauth/authorize";
const GITHUB_TOKEN_URL = "https://github.com/login/oauth/access_token";
const GITHUB_USER_URL = "https://api.github.com/user";
const GITHUB_FETCH_TIMEOUT_MS = 10_000;

export function githubAuthorizeUrl(options: {
  clientId: string;
  redirectUri: string;
  state: string;
}): string {
  const url = new URL(GITHUB_AUTHORIZE_URL);
  url.searchParams.set("client_id", options.clientId);
  url.searchParams.set("redirect_uri", options.redirectUri);
  url.searchParams.set("state", options.state);
  url.searchParams.set("scope", "read:user");
  return url.href;
}

export async function exchangeCodeForToken(options: {
  clientId: string;
  clientSecret: string;
  code: string;
}): Promise<string> {
  const response = await fetch(GITHUB_TOKEN_URL, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
    },
    body: JSON.stringify({
      client_id: options.clientId,
      client_secret: options.clientSecret,
      code: options.code,
    }),
    signal: AbortSignal.timeout(GITHUB_FETCH_TIMEOUT_MS),
  });
  if (!response.ok) {
    throw new Error(`GitHub token exchange failed: ${response.status}`);
  }
  const body = (await response.json()) as {
    access_token?: string;
    error?: string;
  };
  if (!body.access_token) {
    throw new Error(`GitHub token exchange failed: ${body.error ?? "no access_token"}`);
  }
  return body.access_token;
}

export async function fetchGithubLogin(accessToken: string): Promise<string> {
  const response = await fetch(GITHUB_USER_URL, {
    headers: {
      Authorization: `Bearer ${accessToken}`,
      Accept: "application/vnd.github+json",
      "User-Agent": "tq-mcp",
    },
    signal: AbortSignal.timeout(GITHUB_FETCH_TIMEOUT_MS),
  });
  if (!response.ok) {
    throw new Error(`GitHub user fetch failed: ${response.status}`);
  }
  const body = (await response.json()) as { login?: string };
  if (!body.login) {
    throw new Error("GitHub user fetch failed: no login in response");
  }
  return body.login;
}
