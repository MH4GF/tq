import type { z } from "zod";
import type { DbClient } from "../db/client";
import type { Props } from "../types";

export interface ToolContext {
  db: DbClient;
  props: Props;
}

export interface ToolDef<Input extends z.ZodRawShape> {
  name: string;
  description: string;
  inputSchema: Input;
  annotations?: {
    readOnlyHint?: boolean;
    openWorldHint?: boolean;
  };
  handler: (
    params: z.infer<z.ZodObject<Input>>,
    ctx: ToolContext,
  ) => Promise<string>;
}

export function defineTool<Input extends z.ZodRawShape>(
  def: ToolDef<Input>,
): ToolDef<Input> {
  return def;
}
