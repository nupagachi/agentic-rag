import { describe, expect, it } from "vitest";

import { parseApiUrl } from "./env";

describe("parseApiUrl", () => {
  it("falls back to the local dev API when unset or blank", () => {
    expect(parseApiUrl(undefined)).toBe("http://127.0.0.1:8080");
    expect(parseApiUrl("  ")).toBe("http://127.0.0.1:8080");
  });

  it("normalizes to an origin", () => {
    expect(parseApiUrl("https://rag.example.com/api/")).toBe("https://rag.example.com");
  });

  it("rejects non-http(s) schemes and garbage", () => {
    expect(() => parseApiUrl("javascript:alert(1)")).toThrow();
    expect(() => parseApiUrl("ftp://host")).toThrow();
    expect(() => parseApiUrl("not a url")).toThrow();
  });
});
