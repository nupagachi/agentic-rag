---
name: be-go-standards
description: Convention Go cho api/ - layering handler/service/repo theo domain, chi + pgx + sqlc, lỗi chuẩn problem+json, RBAC, context, logging slog, OpenTelemetry, migration an toàn, model gateway, concurrency, test. Dùng khi viết hoặc review code Go.
user-invocable: false
paths: api/**
---

# Convention Go (api/)

Nền tảng: Effective Go, Go Code Review Comments, Uber Go Style Guide. Phần dưới là quy định riêng của dự án; nếu khác nguồn chung thì theo phần dưới.

## 1. Cấu trúc domain
```
internal/<domain>/
├── handler.go     # HTTP: decode → validate → gọi service → encode. Không chứa logic nghiệp vụ.
├── service.go     # nghiệp vụ, kiểm quyền, transaction. Phụ thuộc interface, không phụ thuộc struct cụ thể.
├── repo.go        # bọc query sqlc. Không có logic nghiệp vụ.
├── model.go       # kiểu domain (không dùng struct sinh ra ở tầng ngoài)
├── routes.go      # đăng ký route + quyền yêu cầu
└── *_test.go
```
- Domain gọi nhau qua interface khai báo **ở phía dùng** (consumer-side interface). Không import vòng.
- Constructor `NewService(deps Deps) *Service`; phụ thuộc truyền vào tường minh, không dùng biến global và không dùng `init()` có side effect.

## 2. HTTP (chi)
- Route khai báo quyền bắt buộc:
  ```go
  r.With(authz.Require(authz.KBRead)).Get("/kbs/{kbID}/documents", h.List)
  ```
  Test `TestAllRoutesDeclarePermission` sẽ fail nếu route thiếu quyền.
- Decode JSON với `DisallowUnknownFields`, giới hạn body bằng `http.MaxBytesReader`, validate bằng `go-playground/validator`.
- Response lỗi dạng **RFC 9457 problem+json**:
  ```json
  { "type", "title", "status", "code", "detail", "request_id" }
  ```
  `code` lấy từ `contracts/errors.yaml`.
- Phân trang bằng cursor (`?cursor=&limit=`, limit tối đa 100). Không dùng offset cho bảng lớn.
- POST có tác dụng phụ đắt (upload, chạy eval, re-index) nhận header `Idempotency-Key`.
- SSE: dùng `platform/httpx.SSEWriter`:
  - flush sau mỗi event;
  - heartbeat 15 giây;
  - hỗ trợ `Last-Event-ID`;
  - dừng khi `ctx.Done()`.

## 3. Lỗi
- Lỗi nghiệp vụ: `apperr.New(code, msg)` hoặc `apperr.Wrap(err, code)`. Handler map sang HTTP ở **một chỗ duy nhất** (middleware).
- Wrap bằng `fmt.Errorf("load kb %s: %w", id, err)`; kiểm bằng `errors.Is` / `errors.As`.
- Không trả lỗi nội bộ (SQL, stack) ra client. Không `panic` trong luồng request. Không bỏ qua lỗi (`_ =` phải có comment giải thích).

## 4. Phân quyền
- Mọi truy cập dữ liệu thuộc KB phải qua `authz.CanAccessKB(ctx, user, kbID, perm)` ở service.
- Truy xuất (retrieval) bắt buộc lọc theo `authz.AllowedKBIDs`. Filter Qdrant phải do server dựng, không bao giờ lấy từ input của client.
- ID trong URL không đủ để chứng minh quyền (chống IDOR). Luôn kiểm quan hệ thuộc về.

## 5. Dữ liệu
- SQL chỉ nằm trong `queries/*.sql` và đi qua sqlc. Cấm nối chuỗi SQL.
- Transaction dùng `db.WithTx(ctx, func(q *gen.Queries) error {...})`. Không gọi mạng ra ngoài (LLM, Qdrant) trong transaction.
- ID dùng UUIDv7. Thời gian là `timestamptz`, lưu UTC.
- **Migration** (golang-migrate):
  - tên file theo timestamp (`migrate create -ext sql -dir migrations -format 20060102150405 <name>`) để các task song song không trùng số;
  - mỗi file có `up` và `down`;
  - theo mẫu expand → migrate → contract qua 2 bản phát hành;
  - tạo index trên bảng lớn bằng `CREATE INDEX CONCURRENTLY` (file riêng, không bọc transaction);
  - không đổi tên hay xóa cột trong cùng bản phát hành có code còn đọc cột đó.
- Thao tác ghi đồng thời Postgres và Qdrant: Postgres là nguồn sự thật. Ghi Postgres trước, Qdrant sau, và có job đối soát.

## 6. Context, đồng thời, timeout
- `ctx context.Context` luôn là tham số đầu. Không lưu ctx vào struct.
- Mọi lời gọi ra ngoài có timeout: DB 5 giây, Qdrant 3 giây, LLM theo cấu hình gateway.
- Dùng `errgroup.WithContext` cho việc song song; giới hạn số goroutine bằng `SetLimit`. Không để goroutine rò rỉ: mọi goroutine phải có đường thoát khi ctx bị hủy.
- Dữ liệu chia sẻ: ưu tiên truyền qua channel hoặc giữ bất biến. Dùng mutex thì đặt ngay cạnh field nó bảo vệ.

## 7. LLM và model gateway
- Domain chỉ phụ thuộc interface `llm.ChatModel`, `llm.Embedder`, `llm.Reranker`, lấy qua `gateway`. Cấm import SDK của provider ngoài `internal/llm/<provider>`.
- Mỗi lời gọi ghi lại: provider, model, số token vào/ra, chi phí, độ trễ, `trace_id`.
- Retry với backoff + jitter chỉ cho lỗi 429/5xx. Tôn trọng `Retry-After`.
- Prompt đặt trong `internal/agent/prompts/*.tmpl`, có phiên bản. Nội dung tài liệu được chèn vào prompt luôn nằm trong vùng phân định riêng.

## 8. Quan sát (observability)
- Log bằng `log/slog` dạng JSON, có các field `request_id`, `trace_id`, `user_id`, `kb_id`.
  - Mức `info` cho sự kiện nghiệp vụ, `debug` cho chi tiết.
  - **Không log** mật khẩu, token, API key, nội dung prompt hay tài liệu ở mức info.
- OpenTelemetry: tạo span cho mỗi method service và mỗi lời gọi ra ngoài; trace_id truyền qua job Redis.
- Metric Prometheus đặt tên `rag_<domain>_<name>_<unit>`, ví dụ `rag_retrieval_latency_seconds`.

## 9. Test
- Test dạng bảng (table-driven) với `t.Run`; chạy `t.Parallel()` khi an toàn. Assert bằng `testify/require`.
- Unit test service dùng fake hoặc stub tự viết (không cần framework mock). Handler test bằng `httptest`.
- Test tích hợp gắn build tag `integration`, dùng testcontainers-go (Postgres 16, Redis 7.2, Qdrant 1.11).
- Luôn chạy với `-race`. Coverage package `service` ≥ 80%.
- Test bắt buộc với endpoint mới:
  - đúng quyền → 200;
  - sai quyền → 403;
  - KB của workspace khác → 404;
  - input sai → 400.

## 10. Lệnh
`make check-api` = `gofmt -l` (phải rỗng) + `golangci-lint run` (bật errcheck, govet, staticcheck, gosec, revive, bodyclose, sqlclosecheck, contextcheck, errorlint) + `go test -race ./...` + `sqlc diff`.
