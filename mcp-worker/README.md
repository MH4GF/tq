# tq-mcp-worker

Remote MCP server for tq, running on Cloudflare Workers.

Exposes tq data over MCP (Streamable HTTP) so MCP clients like Claude.ai (Web), Claude Desktop, and Claude Code can read the tq database. Auth is GitHub OAuth via `@cloudflare/workers-oauth-provider`; the local tq CLI keeps writing to the same Turso database independently.

## Architecture

- `src/index.ts` — `OAuthProvider` entry. `/mcp` is the protected API route; everything else falls through to the Hono app.
- `src/app.ts` — `/authorize` (redirects to GitHub), `/callback` (exchanges the code, applies the allowlist, completes the grant), `/health`.
- `src/mcp/handler.ts` — allowlist gate + `createMcpHandler` (from `agents/mcp`, stateless, no Durable Objects).
- `src/mcp/server.ts` — builds the `McpServer` and registers tools.
- `src/mcp/tools/` — one file per tool. v1 ships `list_projects` only.
- `src/db/client.ts` — libsql client for the Turso database (`@libsql/client/web`).

Authorization is a single-user allowlist (`src/mcp/allowlist.ts`); only the GitHub login `MH4GF` is accepted. Tokens never reach tools for other users: the callback rejects them before a grant is created, and the MCP handler re-checks on every request.

## Development

```sh
bun install
bun run test
bun run typecheck
```

### Local server

1. Create a GitHub OAuth App with callback `http://localhost:8787/callback`.
2. Copy `.dev.vars.example` to `.dev.vars` and fill in all values.
3. `bun run dev` (starts wrangler dev on http://localhost:8787).
4. Connect with MCP Inspector: `npx @modelcontextprotocol/inspector` and point it at `http://localhost:8787/mcp`. The OAuth flow opens GitHub; after authorizing, call `list_projects`.

## Deploy

1. Create a production GitHub OAuth App with callback `https://<worker-domain>/callback`.
2. Create the KV namespace and put its id into `wrangler.jsonc`:

   ```sh
   bun run wrangler kv namespace create OAUTH_KV
   ```

3. Set secrets:

   ```sh
   bun run wrangler secret put GITHUB_CLIENT_ID
   bun run wrangler secret put GITHUB_CLIENT_SECRET
   bun run wrangler secret put TURSO_URL
   bun run wrangler secret put TURSO_TOKEN
   ```

4. `bun run deploy`

See `docs/mcp.md` in the repository root for client setup (Claude.ai / Claude Desktop / Claude Code).
