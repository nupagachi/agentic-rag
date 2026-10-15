# contracts/

Hợp đồng dữ liệu dùng chung giữa Go (`server/`), Next.js (`web/`) và Python (`workers/`).
Nguyên tắc **contract-first**: đổi API thì sửa file ở đây trước, code hai phía theo sau.

| File | Nội dung | Ai dùng |
|---|---|---|
| `openapi.yaml` | API REST (OpenAPI 3.0.3) | FE sinh type TS; BE phục vụ Swagger UI, test route khớp spec |

## Xem tài liệu API

Chạy server (`cd server && go run ./cmd/app`), mở:

- Swagger UI: <http://localhost:8080/api/docs/> (qua Nginx: `https://<domain>/api/docs/`)
- Spec thô: <http://localhost:8080/api/openapi.yaml>

Tắt ở môi trường không muốn lộ tài liệu: `API_DOCS_ENABLED=false`.

Không cần chạy server: mở `openapi.yaml` bằng extension *OpenAPI (Swagger) Editor* của VS Code,
hoặc `npx @redocly/cli preview-docs contracts/openapi.yaml`.

## Luồng làm việc BE ↔ FE

```
1. Thiết kế   Ai cần endpoint mới mở PR sửa contracts/openapi.yaml
              (path, request, response, mã lỗi, ví dụ). BE + FE cùng review.
2. Merge spec Sau khi merge, hai bên làm song song:
                FE: sinh lại type → code UI với dữ liệu mock theo `example` trong spec
                BE: go generate ./internal/apidocs → viết handler đúng spec
3. Kiểm tra   CI chạy `go test ./...` ở server/:
                - TestSpecValid       spec hợp lệ, example khớp schema
                - TestSpecInSync      bản nhúng trong server khớp file này
                - TestRoutesMatchSpec route Gin ⇄ path trong spec (thiếu bên nào cũng fail)
```

Đổi không tương thích (xóa trường, đổi kiểu, đổi nghĩa mã lỗi) phải ghi rõ trong mô tả PR
và FE cập nhật cùng lúc. Thêm trường mới (optional) thì không phá FE.

## Phía FE: sinh type TypeScript

Dùng [`openapi-typescript`](https://openapi-ts.dev) (chỉ sinh type, không sinh code,
nên dùng được với axios + react-query đang có):

```bash
cd web
pnpm add -D openapi-typescript
pnpm openapi-typescript ../contracts/openapi.yaml -o src/lib/api/schema.d.ts
```

Gợi ý thêm script `"gen:api"` vào `web/package.json` với lệnh trên. Commit file sinh ra để
review thấy diff. Dùng:

```ts
import type { components, paths } from "@/lib/api/schema";

type ErrorResponse = components["schemas"]["ErrorResponse"];
type Readiness =
  paths["/readyz"]["get"]["responses"]["200"]["content"]["application/json"];
```

## Quy ước FE cần biết

Chi tiết nằm trong phần `info.description` của spec. Tóm tắt:

- **Base URL**: API nghiệp vụ dưới `/api/v1`, cùng origin (Nginx chuyển `/api/` vào Go).
- **JSON** `snake_case`, thời gian RFC 3339, ID là UUID.
- **Lỗi** luôn có dạng:

  ```json
  { "error": { "code": "not_found", "message": "Không tìm thấy", "details": {}, "request_id": "..." } }
  ```

  Rẽ nhánh theo `error.code` (danh sách ở schema `ErrorCode`), hiển thị `error.message`.
  `validation_failed` có lỗi từng trường ở `error.details.fields`.
- **`X-Request-ID`** có trong mọi response: hiển thị/log khi báo lỗi để BE tra log.
- **Xác thực** (từ M1): `Authorization: Bearer <access_token>`.

Interceptor axios mẫu:

```ts
api.interceptors.response.use(undefined, (err) => {
  const body = err.response?.data as ErrorResponse | undefined;
  if (body?.error) {
    return Promise.reject({ ...body.error, status: err.response.status });
  }
  return Promise.reject({ code: "network_error", message: "Không kết nối được máy chủ" });
});
```

## Phía BE: thêm endpoint

1. Sửa `contracts/openapi.yaml`.
2. `cd server && go generate ./internal/apidocs` (chép spec vào bản nhúng).
3. Viết handler, đăng ký route; trả lỗi bằng `httpx.Error(c, httpx.ErrXxx)` để đúng dạng `ErrorResponse`.
4. `go test ./...` — `TestRoutesMatchSpec` báo nếu route và spec lệch nhau.
