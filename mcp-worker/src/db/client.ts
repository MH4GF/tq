import { createClient } from "@libsql/client/web";
import type { Client } from "@libsql/client/web";
import type { Env } from "../types";

export type DbClient = Client;

export function createDbClient(env: Env): DbClient {
  return createClient({ url: env.TURSO_URL, authToken: env.TURSO_TOKEN });
}
