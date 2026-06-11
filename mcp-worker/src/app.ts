import type { AuthRequest } from "@cloudflare/workers-oauth-provider";
import { Hono } from "hono";
import { isAllowedLogin } from "./mcp/allowlist";
import {
  exchangeCodeForToken,
  fetchGithubLogin,
  githubAuthorizeUrl,
} from "./oauth/github";
import type { AppEnv, Props } from "./types";

const app = new Hono<{ Bindings: AppEnv }>();

app.get("/health", (c) => c.json({ ok: true }));

// MCP client lands here; we bounce straight to GitHub, carrying the parsed
// auth request through the OAuth state param.
app.get("/authorize", async (c) => {
  const authRequest = await c.env.OAUTH_PROVIDER.parseAuthRequest(c.req.raw);
  const state = btoa(JSON.stringify(authRequest));
  return c.redirect(
    githubAuthorizeUrl({
      clientId: c.env.GITHUB_CLIENT_ID,
      redirectUri: new URL("/callback", c.req.url).href,
      state,
    }),
  );
});

app.get("/callback", async (c) => {
  const code = c.req.query("code");
  const state = c.req.query("state");
  if (!code || !state) {
    return c.text("Missing code or state", 400);
  }

  let authRequest: AuthRequest;
  try {
    authRequest = JSON.parse(atob(state)) as AuthRequest;
  } catch {
    return c.text("Invalid state", 400);
  }

  const accessToken = await exchangeCodeForToken({
    clientId: c.env.GITHUB_CLIENT_ID,
    clientSecret: c.env.GITHUB_CLIENT_SECRET,
    code,
  });
  const login = await fetchGithubLogin(accessToken);
  if (!isAllowedLogin(login)) {
    return c.text("Forbidden", 403);
  }

  const props: Props = { login, accessToken };
  const { redirectTo } = await c.env.OAUTH_PROVIDER.completeAuthorization({
    request: authRequest,
    userId: login,
    metadata: {},
    scope: [],
    props,
  });
  return c.redirect(redirectTo);
});

export default app;
