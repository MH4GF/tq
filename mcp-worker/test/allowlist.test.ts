import { describe, expect, it } from "vitest";
import { isAllowedLogin } from "../src/mcp/allowlist";

describe("isAllowedLogin", () => {
  it.each([
    { login: "MH4GF", expected: true },
    { login: "mh4gf", expected: false },
    { login: "someone-else", expected: false },
    { login: "", expected: false },
    { login: undefined, expected: false },
  ])("returns $expected for $login", ({ login, expected }) => {
    expect(isAllowedLogin(login)).toBe(expected);
  });
});
