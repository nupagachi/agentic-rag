/**
 * Locale policy. The locale is NOT part of the URL: it lives in a cookie so the
 * user can switch language without a full page reload (see set-locale.ts).
 */
export const locales = ["vi", "en"] as const;

export type Locale = (typeof locales)[number];

export const defaultLocale: Locale = "vi";

export const LOCALE_COOKIE = "NEXT_LOCALE";

/** Feature message files under messages/<locale>/. Add a name when a feature adds a file. */
export const messageFeatures = ["common"] as const;

export function isLocale(value: unknown): value is Locale {
  return typeof value === "string" && (locales as readonly string[]).includes(value);
}

/** SR-15: untrusted input (cookie) is checked against the allowlist; anything else falls back to vi. */
export function resolveLocale(value: string | null | undefined): Locale {
  return isLocale(value) ? value : defaultLocale;
}
