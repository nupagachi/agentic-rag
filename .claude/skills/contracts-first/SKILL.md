---
name: contracts-first
description: Quy tắc contracts-first của monorepo - contracts/ là nguồn sự thật cho API REST, API nội bộ, job Redis, event SSE, mã lỗi và tokenizer BM25; sinh code bằng make gen; quy tắc tương thích ngược. Dùng khi task đụng tới API, job, event hoặc dữ liệu đi giữa web, api, workers.
user-invocable: false
---

# Contracts-first

## Vì sao cần
Ba ngôn ngữ (TypeScript, Go, Python) dùng chung dữ liệu. Nếu mỗi bên tự định nghĩa DTO thì sớm muộn sẽ lệch. Khi chốt hợp đồng trước và sinh code từ đó, FE và BE có thể **làm song song** ngay sau khi PR contract được merge.

## Nguồn và code sinh ra
| Nguồn | Sinh ra | Công cụ |
| --- | --- | --- |
| `contracts/openapi.yaml` | `web/lib/api/gen/` (type + client), `api/gen/openapi/` (type + server interface) | `openapi-typescript` + `openapi-fetch`, `oapi-codegen` |
| `contracts/internal-api.yaml` | `api/gen/internal/`, `workers/rag_worker/clients/gen/` | `oapi-codegen`, `datamodel-code-generator` |
| `contracts/jobs/*.schema.json` | `api/gen/jobs/`, `workers/rag_worker/contracts/` | `go-jsonschema`, `datamodel-code-generator` |
| `contracts/events/*.schema.json` | `web/lib/sse/gen/`, `api/gen/events/` | `json-schema-to-typescript`, `go-jsonschema` |
| `contracts/errors.yaml` | hằng số mã lỗi cho Go, Python, TS + key i18n | script `tools/gen-errors` |
| `contracts/bm25/` | (không sinh code) đặc tả + `test_vectors.json` | test ở cả Go và Python |

`make gen` sinh lại tất cả. CI chạy `make gen && git diff --exit-code`, nên code sinh ra bị lệch thì PR bị chặn.

## Quy trình
1. Mọi thay đổi `contracts/` nằm trong **task contract riêng** (`area: contracts`, `owner: be-agent`, `wave: 0`). PR này nhỏ, Owner review kỹ, merge trước.
2. Task FE và BE ở wave sau chỉ dùng code sinh ra. Thiếu field thì báo `SCOPE_CHANGE_NEEDED`, không tự thêm.

## Quy ước API
- Đường dẫn: `/api/v1/<danh-từ-số-nhiều>`, tên kebab-case, ví dụ `/api/v1/kbs/{kbId}/documents`. Field JSON đặt theo `camelCase`.
- ID: chuỗi UUIDv7. Thời gian: RFC 3339 UTC. Tiền: số nguyên micro-USD.
- Danh sách trả về `{ items, nextCursor }`.
- Lỗi theo RFC 9457 problem+json, có `code` thuộc `errors.yaml`. Mỗi mã có thông điệp vi/en.
- Mỗi operation có `operationId`, `security`, `x-permission` (quyền RBAC yêu cầu) và ví dụ.

## Tương thích ngược
- **Được phép** trong v1:
  - thêm endpoint;
  - thêm field tùy chọn trong response;
  - thêm field tùy chọn trong request;
  - thêm giá trị enum, nhưng client phải xử lý được giá trị lạ.
- **Breaking** (cần Owner duyệt và có ADR trong `docs/adr/`):
  - xóa hoặc đổi tên field;
  - đổi kiểu;
  - thêm field bắt buộc trong request;
  - đổi nghĩa.

  Khi breaking thì làm `v2` hoặc đánh dấu `deprecated` và giữ song song ít nhất một bản phát hành.
- Job Redis có field `schemaVersion`. Worker phải đọc được bản N và N-1.
