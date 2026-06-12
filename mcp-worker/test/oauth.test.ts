import { afterEach, describe, expect, it, vi } from "vitest";
import {
  exchangeCodeForToken,
  fetchGithubLogin,
  githubAuthorizeUrl,
} from "../src/oauth/github";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("githubAuthorizeUrl", () => {
  it("builds the authorize URL with client_id, redirect_uri, and state", () => {
    const url = new URL(
      githubAuthorizeUrl({
        clientId: "cid",
        redirectUri: "https://example.com/callback",
        state: "abc123",
      }),
    );
    expect(url.origin + url.pathname).toBe(
      "https://github.com/login/oauth/authorize",
    );
    expect(url.searchParams.get("client_id")).toBe("cid");
    expect(url.searchParams.get("redirect_uri")).toBe(
      "https://example.com/callback",
    );
    expect(url.searchParams.get("state")).toBe("abc123");
    expect(url.searchParams.get("scope")).toBe("read:user");
  });
});

describe("exchangeCodeForToken", () => {
  it("returns the access token on success", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => Response.json({ access_token: "gh-token" })),
    );
    const token = await exchangeCodeForToken({
      clientId: "cid",
      clientSecret: "secret",
      code: "code123",
    });
    expect(token).toBe("gh-token");
  });

  it("throws when GitHub responds without access_token", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => Response.json({ error: "bad_verification_code" })),
    );
    await expect(
      exchangeCodeForToken({
        clientId: "cid",
        clientSecret: "secret",
        code: "expired",
      }),
    ).rejects.toThrow("bad_verification_code");
  });

  it("throws on non-2xx response", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("nope", { status: 500 })),
    );
    await expect(
      exchangeCodeForToken({
        clientId: "cid",
        clientSecret: "secret",
        code: "code123",
      }),
    ).rejects.toThrow("500");
  });
});

describe("fetchGithubLogin", () => {
  it("returns the login on success", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => Response.json({ login: "MH4GF" })),
    );
    expect(await fetchGithubLogin("gh-token")).toBe("MH4GF");
  });

  it("sends the token as a Bearer header", async () => {
    const fetchMock = vi.fn(async () => Response.json({ login: "MH4GF" }));
    vi.stubGlobal("fetch", fetchMock);
    await fetchGithubLogin("gh-token");
    const [, init] = fetchMock.mock.calls[0] as unknown as [
      string,
      RequestInit,
    ];
    expect(
      new Headers(init.headers).get("Authorization"),
    ).toBe("Bearer gh-token");
  });

  it("throws when login is missing", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => Response.json({})));
    await expect(fetchGithubLogin("gh-token")).rejects.toThrow("no login");
  });
});
