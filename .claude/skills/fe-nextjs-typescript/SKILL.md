---
name: fe-nextjs-typescript
description: Convention Next.js 15 App Router + TypeScript strict của dự án - cấu trúc thư mục, Server/Client component, gọi API qua client sinh từ OpenAPI, TanStack Query, form, SSE, i18n, test FE. Dùng khi viết hoặc review code trong web/.
user-invocable: false
paths: web/**
---

# Convention Next.js + TypeScript

## 1. Cấu trúc
```
web/
├── app/(auth)/...            # trang không có sidebar
├── app/(app)/layout.tsx      # khung app: sidebar + inset
├── app/(app)/<feature>/page.tsx
├── components/ui/            # shadcn (sinh bằng CLI)
├── components/<feature>/     # component theo tính năng: chat/, kb/, inspector/, settings/
├── features/<feature>/       # hooks (useXxxQuery), schema zod, mapper; không chứa JSX lớn
├── lib/api/                  # client sinh từ openapi + fetcher (refresh token)
├── lib/sse/                  # EventSource + reconnect Last-Event-ID
├── messages/{vi,en}/<feature>.json   # tách theo tính năng để task song song không xung đột
├── e2e/                      # Playwright
└── tests/                    # setup Vitest, MSW handlers
```
- Tên file: `kebab-case.tsx`. Tên component: `PascalCase`. Tên hook: `useCamelCase`.
- Mỗi file có một component export chính. Dùng named export, trừ `page.tsx`, `layout.tsx` và các file đặc biệt của Next.
- Import tuyệt đối qua alias `@/`. Không import ngược từ `components/` vào `lib/`.

## 2. TypeScript
- `strict: true`, `noUncheckedIndexedAccess: true`, `exactOptionalPropertyTypes: true`.
- Không dùng `any`. Dữ liệu chưa rõ kiểu dùng `unknown` rồi thu hẹp bằng zod.
- Type của API **chỉ** lấy từ `@/lib/api/gen`. Không định nghĩa lại DTO bằng tay.
- Union có tên phân biệt (discriminated union) cho trạng thái, kiểm đủ trường hợp bằng `satisfies never`.
- Props khai báo bằng `type`. Chỉ dùng `interface` khi cần mở rộng.

## 3. Server và Client component
- Theo kiến trúc, **trình duyệt gọi `/api` trực tiếp**; Next.js server không gọi backend.
  - Server Component chỉ dùng cho khung tĩnh: layout, metadata, i18n.
  - Phần lấy dữ liệu là Client Component (`"use client"`) dùng TanStack Query.
- Đặt `"use client"` ở lá thấp nhất có thể.
- Không dùng Server Actions hay Route Handlers để proxy API.

## 4. Gọi API
- Dùng `apiClient` trong `lib/api`. Nó tự gắn access token (lưu trong bộ nhớ, không lưu localStorage); gặp 401 thì refresh một lần qua cookie httpOnly rồi gọi lại.
- Lỗi trả về là `ApiError { code, status, detail }`. Mã lỗi theo `contracts/errors.yaml`; thông điệp hiển thị lấy qua `t(\`errors.${code}\`)`.
- TanStack Query:
  - Query key theo mẫu `[feature, id, sub, params]`, ví dụ `['kb', kbId, 'documents', { status, cursor }]`. Khai báo tập trung trong `features/<f>/keys.ts`.
  - `staleTime` mặc định 30 giây. Trạng thái pipeline realtime cập nhật qua SSE bằng `queryClient.setQueryData`, không polling dày.
  - Mutation luôn `invalidateQueries` đúng key. Chỉ cập nhật lạc quan (optimistic update) khi có rollback.
- Upload file lớn: xin presigned URL từ API, PUT thẳng lên MinIO, theo dõi tiến độ bằng XHR. Không gửi file qua `/api`.

## 5. Form
`react-hook-form` + `zodResolver`. Schema đặt trong `features/<f>/schema.ts`, map lỗi server vào field qua `setError`. Nút submit có trạng thái pending và chặn submit hai lần.

## 6. SSE và streaming chat
- Dùng `lib/sse`: tự reconnect với `Last-Event-ID`, đóng kết nối khi unmount, hủy được bằng AbortController.
- Event theo `contracts/events/*.schema.json`, parse bằng zod. Event không hợp lệ thì log và bỏ qua, không làm crash UI.

## 7. Render nội dung từ LLM
- Dùng `react-markdown` + `remark-gfm` + `rehype-sanitize`. **Cấm** raw HTML và `dangerouslySetInnerHTML`.
- Citation `[n]` là component riêng. Link ra ngoài có `rel="noopener noreferrer"` và `target="_blank"`.

## 8. Hiệu năng
- `react-pdf` và các thư viện nặng nạp bằng `next/dynamic`. Ảnh dùng `next/image`.
- Danh sách dài thì virtualize. Không lấy dữ liệu theo kiểu thác nước giữa cha và con.
- Mục tiêu: LCP < 2.5s trên mạng LAN, bundle mỗi route < 200KB gzip (theo dõi bằng `@next/bundle-analyzer`).

## 9. Test
- **Vitest + Testing Library:**
  - test hành vi người dùng (`getByRole`, `userEvent`), không test state nội bộ;
  - MSW mock API theo type sinh từ OpenAPI.
- **Playwright** (`web/e2e`):
  - chạy với stack dev và mock LLM provider;
  - selector ưu tiên theo role; `data-testid` chỉ dùng khi không còn cách nào khác.
- Mỗi AC giao diện có ít nhất một test.

## 10. Lệnh
`make check-web` = `npm run lint` (eslint + `next/core-web-vitals` + `@typescript-eslint/strict`) + `npm run typecheck` (`tsc --noEmit`) + `npm test` + `npm run build`.
