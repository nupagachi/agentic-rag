import { afterEach, describe, expect, it } from "vitest";

import { LOCALE_COOKIE, defaultLocale, isLocale, locales, resolveLocale } from "./config";
import { loadMessages } from "./messages";
import { buildLocaleCookie, setLocaleCookie } from "./set-locale";

function keyPaths(value: unknown, prefix = ""): string[] {
  if (typeof value !== "object" || value === null) return [prefix];
  return Object.entries(value).flatMap(([key, child]) => keyPaths(child, prefix === "" ? key : `${prefix}.${key}`));
}

describe("resolveLocale (SR-15)", () => {
  it("defaults to vi", () => {
    expect(defaultLocale).toBe("vi");
    expect(resolveLocale(undefined)).toBe("vi");
    expect(resolveLocale(null)).toBe("vi");
    expect(resolveLocale("")).toBe("vi");
  });

  it("accepts allowlisted locales", () => {
    expect(resolveLocale("vi")).toBe("vi");
    expect(resolveLocale("en")).toBe("en");
  });

  it("falls back to vi for values outside the allowlist", () => {
    expect(resolveLocale("xx")).toBe("vi");
    expect(resolveLocale("EN")).toBe("vi");
    expect(resolveLocale("../../etc/passwd")).toBe("vi");
    expect(resolveLocale("__proto__")).toBe("vi");
    expect(isLocale("fr")).toBe(false);
  });

  it("keeps the allowlist at exactly vi and en", () => {
    expect([...locales]).toEqual(["vi", "en"]);
  });
});

describe("locale cookie", () => {
  afterEach(() => {
    document.cookie = `${LOCALE_COOKIE}=; Path=/; Max-Age=0`;
  });

  it("builds a Lax, path-wide, one-year cookie", () => {
    const cookie = buildLocaleCookie("en");
    expect(cookie).toContain(`${LOCALE_COOKIE}=en`);
    expect(cookie).toContain("Path=/");
    expect(cookie).toContain("SameSite=Lax");
    expect(cookie).toContain("Max-Age=31536000");
  });

  it("writes the cookie in the browser without navigation", () => {
    setLocaleCookie("en");
    expect(document.cookie).toContain(`${LOCALE_COOKIE}=en`);
  });

  it("ignores locales outside the allowlist", () => {
    // @ts-expect-error deliberately passing a value that the type system forbids
    setLocaleCookie("xx");
    expect(document.cookie).not.toContain("xx");
  });
});

describe("messages", () => {
  it("vi and en expose the same keys per feature", async () => {
    const [vi, en] = await Promise.all([loadMessages("vi"), loadMessages("en")]);
    expect(keyPaths(vi).sort()).toEqual(keyPaths(en).sort());
    expect(Object.keys(vi)).toContain("common");
  });
});
