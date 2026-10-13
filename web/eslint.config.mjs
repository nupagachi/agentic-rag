import { dirname } from "node:path";
import { fileURLToPath } from "node:url";

import { FlatCompat } from "@eslint/eslintrc";
import js from "@eslint/js";
import i18next from "eslint-plugin-i18next";
import tseslint from "typescript-eslint";

const compat = new FlatCompat({ baseDirectory: dirname(fileURLToPath(import.meta.url)) });

const USER_VISIBLE_ATTRIBUTES = [
  "aria-label",
  "aria-description",
  "aria-roledescription",
  "aria-valuetext",
  "alt",
  "title",
  "placeholder",
  "label",
  "description",
  "tooltip",
  "text",
];

const config = [
  {
    ignores: [".next/**", "node_modules/**", "next-env.d.ts", "coverage/**", "lib/api/gen/**", "lib/sse/gen/**"],
  },
  js.configs.recommended,
  ...compat.extends("next/core-web-vitals", "next/typescript"),
  ...tseslint.configs.strict,
  {
    rules: {
      // SR-19: never render raw HTML from the model or users.
      "react/no-danger": "error",
      "@typescript-eslint/no-explicit-any": "error",
      "@typescript-eslint/ban-ts-comment": "error",
      "no-console": "error",
      // SR-14: process.env is read only in lib/env.ts (overridden below for that file).
      "no-restricted-properties": [
        "error",
        { object: "process", property: "env", message: "Read configuration through @/lib/env only." },
      ],
    },
  },
  {
    files: ["lib/env.ts"],
    rules: { "no-restricted-properties": "off" },
  },
  // AC5/SR: no hardcoded UI strings in app/** and components/** (shadcn primitives and tests excluded).
  {
    files: ["app/**/*.{ts,tsx}", "components/**/*.{ts,tsx}"],
    ignores: ["components/ui/**", "**/*.test.{ts,tsx}"],
    ...i18next.configs["flat/recommended"],
    rules: {
      "i18next/no-literal-string": [
        "error",
        {
          mode: "jsx-only",
          // Check JSX text plus attributes that carry user-visible text; style-like props
          // (variant, side, attribute, className ...) are not translated.
          "jsx-attributes": { include: USER_VISIBLE_ATTRIBUTES },
        },
      ],
    },
  },
  {
    files: ["tests/**", "**/*.test.{ts,tsx}", "vitest.config.ts"],
    rules: { "no-restricted-properties": "off" },
  },
];

export default config;
