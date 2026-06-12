const ALLOWED_LOGINS = new Set(["MH4GF"]);

export function isAllowedLogin(login: string | undefined): boolean {
  return login !== undefined && ALLOWED_LOGINS.has(login);
}
