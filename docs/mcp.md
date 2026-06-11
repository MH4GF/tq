# Remote MCP Server

tq ships a remote MCP server (`mcp-worker/`) that runs on Cloudflare Workers and reads the same Turso database the local tq CLI uses. MCP clients — Claude.ai (Web), Claude Desktop, Claude Code — connect over Streamable HTTP with GitHub OAuth.

v1 exposes a single read-only tool:

| Tool | Description |
|---|---|
| `list_projects` | List tq projects, newest first (`limit` 1–100, default 20) |

The dispatch loop, schedules, and all writes stay in the local tq CLI. The worker is stateless: every request authenticates via OAuth, checks the GitHub login against the allowlist, then queries Turso directly.

## Prerequisites

1. A Turso database initialized by the local tq (running any tq command, e.g. `tq project list`, applies migrations automatically).
2. The deployed worker URL (see `mcp-worker/README.md` for deploy steps), referred to as `https://tq-mcp.example.workers.dev` below.

## Connect from Claude.ai (Web)

1. Settings → Connectors → Add custom connector.
2. Enter `https://tq-mcp.example.workers.dev/mcp`.
3. Claude.ai discovers the OAuth endpoints automatically; complete the GitHub authorization when prompted.
4. Ask Claude: "list my tq projects" — it calls `list_projects`.

## Connect from Claude Code

```sh
claude mcp add tq --transport http https://tq-mcp.example.workers.dev/mcp
```

Then run `/mcp` inside a session to complete the OAuth flow.

## Connect from Claude Desktop

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "tq": {
      "command": "npx",
      "args": ["mcp-remote", "https://tq-mcp.example.workers.dev/mcp"]
    }
  }
}
```

`mcp-remote` opens a browser window for the GitHub OAuth flow on first use.

## Authorization model

Authentication is GitHub OAuth, but authorization is a hard-coded allowlist (`mcp-worker/src/mcp/allowlist.ts`). Only the GitHub login `MH4GF` can complete the flow; other users receive 403 at the OAuth callback and again at the MCP route. Multi-user support (per-user Turso databases) is out of scope for v1.

## Local development

See `mcp-worker/README.md`. In short: create a dev GitHub OAuth App pointing at `http://localhost:8787/callback`, fill `.dev.vars`, run `bun run dev`, and connect with MCP Inspector (`npx @modelcontextprotocol/inspector`).
