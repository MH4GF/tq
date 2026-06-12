import type { AuthRequest, ClientInfo } from "@cloudflare/workers-oauth-provider";

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

export function renderApprovalDialog(options: {
  client: ClientInfo | null;
  authRequest: AuthRequest;
  signedState: string;
}): string {
  const clientName = escapeHtml(options.client?.clientName ?? "Unknown client");
  const redirectUri = escapeHtml(options.authRequest.redirectUri);
  const state = escapeHtml(options.signedState);
  return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Authorize tq MCP</title>
<style>
  body { font-family: system-ui, sans-serif; max-width: 32rem; margin: 4rem auto; padding: 0 1rem; }
  .card { border: 1px solid #ddd; border-radius: 8px; padding: 1.5rem; }
  dt { color: #666; font-size: 0.8rem; margin-top: 0.75rem; }
  dd { margin: 0; font-family: ui-monospace, monospace; word-break: break-all; }
  button { margin-top: 1.5rem; padding: 0.6rem 1.2rem; font-size: 1rem; cursor: pointer; }
</style>
</head>
<body>
<div class="card">
<h1>Authorize access to tq</h1>
<p>This client is requesting access to your tq projects via MCP.</p>
<dl>
<dt>Client</dt><dd>${clientName}</dd>
<dt>Redirect URI</dt><dd>${redirectUri}</dd>
</dl>
<form method="post" action="/authorize">
<input type="hidden" name="state" value="${state}">
<button type="submit">Approve and continue to GitHub</button>
</form>
</div>
</body>
</html>`;
}
