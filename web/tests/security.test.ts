import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { dirname, join, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";

import { ESLint } from "eslint";
import { describe, expect, it } from "vitest";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");

type PackageJson = {
  scripts: Record<string, string>;
  dependencies: Record<string, string>;
};

function readPackage(): PackageJson {
  return JSON.parse(readFileSync(join(root, "package.json"), "utf8")) as PackageJson;
}

function walk(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const full = join(dir, name);
    if (name === "node_modules" || name === ".next") return [];
    return statSync(full).isDirectory() ? walk(full) : [full];
  });
}

function stripComments(source: string): string {
  return source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");
}

function atLeast(version: string, min: [number, number, number]): boolean {
  const parts = version.replace(/^[^\d]*/, "").split(".").map((p) => Number.parseInt(p, 10));
  const [a = 0, b = 0, c = 0] = parts;
  const [x, y, z] = min;
  return a !== x ? a > x : b !== y ? b > y : c >= z;
}

describe("SR-5: dev server binds loopback only", () => {
  it("dev script is `next dev -H 127.0.0.1`", () => {
    expect(readPackage().scripts.dev).toBe("next dev -H 127.0.0.1");
  });

  it("next.config has no allowedDevOrigins", () => {
    const config = stripComments(readFileSync(join(root, "next.config.ts"), "utf8"));
    expect(config).not.toMatch(/allowedDevOrigins/);
  });
});

describe("SR-14: process.env only in lib/env.ts", () => {
  it("next.config has no `env` key", () => {
    const config = stripComments(readFileSync(join(root, "next.config.ts"), "utf8"));
    expect(config).not.toMatch(/\benv\s*:/);
  });

  it("no source file outside lib/env.ts reads process.env", () => {
    const offenders = ["app", "components", "lib", "i18n"]
      .flatMap((dir) => walk(join(root, dir)))
      .filter((file) => /\.(ts|tsx|js|jsx|mjs)$/.test(file))
      .filter((file) => !/\.test\.(ts|tsx)$/.test(file))
      .filter((file) => relative(root, file).split(sep).join("/") !== "lib/env.ts")
      .filter((file) => /process\s*\.\s*env|process\s*\[\s*["']env["']\s*\]/.test(stripComments(readFileSync(file, "utf8"))))
      .map((file) => relative(root, file));
    expect(offenders).toEqual([]);
  });

  it("lib/env.ts reads only NEXT_PUBLIC_API_URL", () => {
    const source = stripComments(readFileSync(join(root, "lib", "env.ts"), "utf8"));
    const reads = [...source.matchAll(/process\.env\.([A-Za-z0-9_]+)/g)].map((m) => m[1]);
    expect(reads).toEqual(["NEXT_PUBLIC_API_URL"]);
    expect(source).not.toMatch(/process\.env\s*[[)]|\.\.\.process\.env/);
  });
});

describe("SR-3 / SR-16: supply chain", () => {
  it("uses a patched Next.js and React, pinned exactly", () => {
    const { dependencies } = readPackage();
    expect(dependencies.next).toMatch(/^\d+\.\d+\.\d+$/);
    expect(dependencies.react).toMatch(/^\d+\.\d+\.\d+$/);
    expect(atLeast(dependencies.next ?? "", [15, 5, 7])).toBe(true);
    // CVE-2025-55182 fixed in 19.0.1, 19.1.2, 19.2.1; any 19.2.x >= 19.2.1 or 19.3+ is fine here.
    const react = dependencies.react ?? "";
    expect(atLeast(react, [19, 2, 1])).toBe(true);
    expect(dependencies["react-dom"]).toBe(react);
  });

  it("commits a lockfile", () => {
    expect(existsSync(join(root, "package-lock.json"))).toBe(true);
  });
});

describe("lint policy (AC5, SR-19)", () => {
  const eslint = new ESLint({ cwd: root });

  async function messagesFor(code: string, file: string): Promise<string[]> {
    const [result] = await eslint.lintText(code, { filePath: join(root, file) });
    return (result?.messages ?? []).map((m) => m.ruleId ?? "");
  }

  it("flags a hardcoded UI string in components/**", async () => {
    const rules = await messagesFor("export const Sample = () => <p>Hello world</p>;\n", "components/sample/a.tsx");
    expect(rules).toContain("i18next/no-literal-string");
  });

  it("flags a hardcoded aria-label in app/**", async () => {
    const rules = await messagesFor('export const Sample = () => <button aria-label="Close dialog" />;\n', "app/sample.tsx");
    expect(rules).toContain("i18next/no-literal-string");
  });

  it("does not apply the string rule to components/ui/**", async () => {
    const rules = await messagesFor("export const Sample = () => <p>Hello world</p>;\n", "components/ui/a.tsx");
    expect(rules).not.toContain("i18next/no-literal-string");
  });

  it("forbids dangerouslySetInnerHTML (react/no-danger)", async () => {
    const rules = await messagesFor(
      "export const Sample = ({ h }: { h: string }) => <div dangerouslySetInnerHTML={{ __html: h }} />;\n",
      "components/ui/a.tsx",
    );
    expect(rules).toContain("react/no-danger");
  });

  it("forbids any and process.env outside lib/env.ts", async () => {
    const rules = await messagesFor("export const x: any = process.env.FOO;\n", "lib/sample.ts");
    expect(rules).toContain("@typescript-eslint/no-explicit-any");
    expect(rules).toContain("no-restricted-properties");
  });
}, 60_000);
