import { Hono } from "hono";
import { renderApprovalDialog } from "./oauth/approval";
import {
  exchangeCodeForToken,
  fetchGithubLogin,
  githubAuthorizeUrl,
} from "./oauth/github";
import { signState, verifyState } from "./oauth/state";
import { isAllowedLogin } from "./mcp/allowlist";
import type { AppEnv, Props } from "./types";

const app = new Hono<{ Bindings: AppEnv }>();

app.get("/health", (c) => c.json({ ok: true }));

app.get("/authorize", async (c) => {
  const authRequest = await c.env.OAUTH_PROVIDER.parseAuthRequest(c.req.raw);
  const client = await c.env.OAUTH_PROVIDER.lookupClient(authRequest.clientId);
  const signedState = await signState(authRequest, c.env.OAUTH_STATE_SECRET);
  return c.html(renderApprovalDialog({ client, authRequest, signedState }));
});

app.post("/authorize", async (c) => {
  const body = await c.req.parseBody();
  const signedState = typeof body.state === "string" ? body.state : "";
  try {
    await verifyState(signedState, c.env.OAUTH_STATE_SECRET);
  } catch {
    return c.text("Invalid state", 400);
  }
  return c.redirect(
    githubAuthorizeUrl({
      clientId: c.env.GITHUB_CLIENT_ID,
      redirectUri: new URL("/callback", c.req.url).href,
      state: signedState,
    }),
  );
});

app.get("/callback", async (c) => {
  const code = c.req.query("code");
  const state = c.req.query("state");
  if (!code || !state) {
    return c.text("Missing code or state", 400);
  }

  let authRequest: Awaited<ReturnType<typeof verifyState>>;
  try {
    authRequest = await verifyState(state, c.env.OAUTH_STATE_SECRET);
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

  const props: Props = { login };
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
