/**
 * Access token lives in memory only (never localStorage) to limit XSS
 * exposure. The refresh token is an httpOnly cookie set by the Go API.
 */
let accessToken: string | null = null;

export const tokenStore = {
  get: () => accessToken,
  set: (token: string | null) => {
    accessToken = token;
  },
  clear: () => {
    accessToken = null;
  },
};
