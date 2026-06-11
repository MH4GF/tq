import OAuthProvider from "@cloudflare/workers-oauth-provider";
import app from "./app";
import mcpHandler from "./mcp/handler";

export default new OAuthProvider({
  apiRoute: "/mcp",
  // @ts-expect-error - OAuthProvider's handler types don't carry our Env
  apiHandler: mcpHandler,
  defaultHandler: app,
  authorizeEndpoint: "/authorize",
  tokenEndpoint: "/token",
  clientRegistrationEndpoint: "/register",
});
