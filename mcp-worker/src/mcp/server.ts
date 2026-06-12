import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import type { ToolCallback } from "@modelcontextprotocol/sdk/server/mcp.js";
import type { z } from "zod";
import type { ToolContext, ToolDef } from "../lib/define-tool";
import listProjects from "./tools/list-projects";

export function buildServer(ctx: ToolContext): McpServer {
  const server = new McpServer(
    { name: "tq-mcp", version: "0.1.0" },
    { capabilities: { tools: {} } },
  );
  registerTool(server, ctx, listProjects);
  return server;
}

function registerTool<Input extends z.ZodRawShape>(
  server: McpServer,
  ctx: ToolContext,
  tool: ToolDef<Input>,
): void {
  const callback = (async (params: z.infer<z.ZodObject<Input>>) => {
    const text = await tool.handler(params, ctx);
    return { content: [{ type: "text" as const, text }] };
  }) as unknown as ToolCallback<Input>;

  server.registerTool(
    tool.name,
    {
      description: tool.description,
      inputSchema: tool.inputSchema,
      annotations: tool.annotations,
    },
    callback,
  );
}
