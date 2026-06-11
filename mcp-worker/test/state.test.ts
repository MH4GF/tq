import type { AuthRequest } from "@cloudflare/workers-oauth-provider";
import { describe, expect, it } from "vitest";
import { signState, verifyState } from "../src/oauth/state";

const SECRET = "test-secret-0123456789";

const authRequest: AuthRequest = {
  responseType: "code",
  clientId: "client-123",
  redirectUri: "https://example.com/callback",
  scope: [],
  state: "client-state",
} as AuthRequest;

describe("state signing", () => {
  it("round-trips an auth request through sign/verify", async () => {
    const signed = await signState(authRequest, SECRET);
    const parsed = await verifyState(signed, SECRET);
    expect(parsed.clientId).toBe("client-123");
    expect(parsed.redirectUri).toBe("https://example.com/callback");
  });

  it("rejects a tampered payload", async () => {
    const signed = await signState(authRequest, SECRET);
    const [payload, signature] = signed.split(".");
    const flipped = `${payload.slice(0, -1)}${payload.at(-1) === "A" ? "B" : "A"}`;
    await expect(
      verifyState(`${flipped}.${signature}`, SECRET),
    ).rejects.toThrow();
  });

  it("rejects a wrong secret", async () => {
    const signed = await signState(authRequest, SECRET);
    await expect(verifyState(signed, "other-secret")).rejects.toThrow(
      "Invalid state signature",
    );
  });

  it("rejects a state without a signature separator", async () => {
    await expect(verifyState("no-dot-here", SECRET)).rejects.toThrow(
      "Malformed state",
    );
  });

  it("preserves non-ASCII characters", async () => {
    const withUnicode = {
      ...authRequest,
      state: "日本語-state-🎌",
    } as AuthRequest;
    const signed = await signState(withUnicode, SECRET);
    const parsed = await verifyState(signed, SECRET);
    expect(parsed.state).toBe("日本語-state-🎌");
  });
});
