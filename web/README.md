# web — Agentic RAG frontend

Next.js 15 (App Router) · TypeScript · Tailwind 4 · shadcn/ui (Base UI) · TanStack Query · axios · react-pdf · next-intl · react-hook-form + zod · moment · lodash.

## Chạy

```bash
corepack enable            # pnpm theo packageManager
pnpm install
cp .env.example .env.local
pnpm dev                   # http://localhost:3000, /api/* proxy tới API_PROXY_TARGET
pnpm check                 # typecheck + lint + format + test (giống CI)
```

Node 22 LTS (`.nvmrc`). Docker: `docker build -t agentic-rag-web ./web` (output `standalone`, user non-root, healthcheck `/healthz`).

## Cấu trúc

```
app/
  (auth)/                  login, register, invite, forgot-password
  (app)/                   app shell (sidebar) — chat, kb, settings, admin
  healthz/                 liveness probe cho container
components/
  ui/                      shadcn (sinh bằng CLI, không sửa tay nếu không cần)
  layout/  providers/      shell, nav, AppProviders (Query, theme, toast)
  auth/ chat/ citation-viewer/ inspector/ settings-form/   theo domain
config/env.ts              env public, validate bằng zod
hooks/                     React Query hooks theo domain (use-auth, ...)
i18n/                      next-intl: locale từ cookie NEXT_LOCALE, không prefix URL
lib/
  api/                     axios client, refresh token single-flight, ApiError, query keys
  api/services/            gọi API theo domain (*.service.ts)
  sse/                     đọc SSE qua fetch: Bearer token, Last-Event-ID, backoff
  validations/             zod schema (message là i18n key)
  date.ts  utils.ts
messages/                  vi.json (nguồn type), en.json
types/                     type dùng chung (thay bằng type sinh từ contracts/openapi.yaml)
```

## Quy ước

- Server Component mặc định; chỉ thêm `"use client"` khi cần state/effect/browser API.
- Gọi API: `service` (thuần, không React) → `hook` (React Query) → component. Không gọi axios trực tiếp trong component.
- Query key lấy từ `lib/api/query-keys.ts`.
- Access token chỉ giữ trong memory; refresh token là cookie httpOnly do Go API đặt.
- Text hiển thị qua `next-intl`; thêm key vào cả `vi.json` và `en.json`.
- PDF viewer dùng `components/citation-viewer` (đã `ssr: false`).
- lodash import theo hàm (`import debounce from "lodash/debounce"`) để giữ bundle nhỏ.
- Test: `*.test.ts` cạnh file; mặc định env `node`, component test thêm `// @vitest-environment jsdom`.
