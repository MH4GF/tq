import { createMcpHandler } from "agents/mcp";
import { createDbClient } from "../db/client";
import type { Env, Props } from "../types";
import { isAllowedLogin } from "./allowlist";
import { buildServer } from "./server";

type OAuthExecutionContext = ExecutionContext & { props?: Props };

const mcpHandler = {
  fetch: async (
    request: Request,
    env: Env,
    ctx: OAuthExecutionContext,
  ): Promise<Response> => {
    const props = ctx.props;
    if (!isAllowedLogin(props?.login)) {
      return new Response("Forbidden", { status: 403 });
    }
    const server = buildServer({ db: createDbClient(env), props: props! });
    const handler = createMcpHandler(server, { route: "/mcp" });
    return handler(request, env, ctx);
  },
};

export default mcpHandler;
