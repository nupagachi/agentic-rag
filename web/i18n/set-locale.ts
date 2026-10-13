import { LOCALE_COOKIE, isLocale, type Locale } from "./config";

const ONE_YEAR_SECONDS = 60 * 60 * 24 * 365;

/** Build the Set-Cookie string for a locale (non-sensitive, readable by the server on the next request). */
export function buildLocaleCookie(locale: Locale): string {
  return `${LOCALE_COOKIE}=${locale}; Path=/; Max-Age=${ONE_YEAR_SECONDS}; SameSite=Lax`;
}

/**
 * Persist the chosen locale. The caller then runs `router.refresh()` so Server
 * Components re-render with the new messages; there is no full page reload.
 */
export function setLocaleCookie(locale: Locale): void {
  if (!isLocale(locale)) return;
  document.cookie = buildLocaleCookie(locale);
}
