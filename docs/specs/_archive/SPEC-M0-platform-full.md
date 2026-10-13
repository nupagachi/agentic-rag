---
id: SPEC-M0-platform
title: Platform — base chạy được trên local từ ngày đầu
owner: "@<owner>"            # thay bằng GitHub username
status: draft
scope_note: "Giai đoạn này chỉ chạy local trên máy dev. DevOps (CI/CD, staging, registry, ký image, backup, alert, triển khai 2 máy) hoãn lại, xem mục 10."
prd_refs: ["7.1 Hiệu năng", "7.2 Bảo mật", "7.4 Vận hành", "8 Kiến trúc"]
p0_module: M0
ui_refs: [Main.dc.html, Settings.dc.html]   # chỉ phần vỏ app: sidebar, avatar menu, Settings popup
docs_refs:
  - "Doc → tab Kiến trúc (mục 3 cấu trúc code, mục 5 Docker)"
  - "Doc → tab Base dự án (thiết kế chi tiết của SPEC này)"
  - "docs/Hướng dẫn setup & workflow agent.md"
---

# SPEC-M0: Platform (local)

## 1. Vấn đề và mục tiêu

**Vấn đề.**
- Chưa có repo, môi trường chạy hay quy ước chung.
- Nếu các module M1–M10 bắt đầu khi chưa có những thứ này, mỗi người hoặc agent sẽ tự dựng hạ tầng theo cách riêng. Hệ quả là code lệch chuẩn, không có trace hay log để gỡ lỗi, và bảo mật phải vá sau.
- Quy trình coding agent cũng chưa chạy được, vì cần repo và `make check-*` có sẵn.

**Mục tiêu đo được.**
1. Máy dev trắng chạy `make dev` lên đủ stack trong **≤ 10 phút** khi làm theo README. Stack dev không cần Internet (sau lần pull image đầu) và không tốn tiền API.
2. **Walking skeleton "echo job"** chạy hết luồng web → Go API → Redis → Python worker → model gateway → mock LLM → SSE về web, trên máy local.
3. `make check` (lint, typecheck, test, gen-drift) chạy trên local trong **≤ 10 phút** và là cổng kiểm duy nhất trước khi mở PR, thay cho CI ở giai đoạn này.
4. Một request hiện thành **một trace liền mạch** từ Nginx tới worker và nhảy được sang log của nó (bật profile `obs` khi cần).
5. Domain mẫu (`system`) và trang mẫu (`/system`) đủ chuẩn để fe-agent, be-agent bắt chước khi làm M1 trở đi.

## 2. Ngoài phạm vi
- Mọi tính năng nghiệp vụ: auth thật (M1), KB (M2), provider và gateway đầy đủ (M3), ingestion (M4)… M0 chỉ dựng **khung** và **một luồng mẫu**.
  - Endpoint mẫu dùng xác thực tạm cho dev (mục 4.1) và sẽ được thay bằng M1.
- **Toàn bộ phần DevOps** (hoãn lại, chi tiết ở mục 10): CI/CD GitHub Actions, ruleset nhánh, môi trường staging/production, Harbor, build/ký/quét image, SBOM, Renovate, e2e hằng đêm, dashboard và alert, runbook, backup/khôi phục, triển khai 2 máy chủ.
- Kubernetes, HA nhiều node, failover tự động.
- mTLS giữa các service nội bộ → ghi ADR, làm ở Phase 2.
- Vault. Local dùng Docker secrets dạng file.

## 3. User story và acceptance criteria

Vai trò trong SPEC này:
- **Dev**: kỹ sư hoặc coding agent.
- **Owner**: người phụ trách dự án.
- **System Admin**: theo PRD mục 3.

### 3.1 Repo và quy ước: là Dev, tôi muốn một monorepo có quy ước và bộ lệnh thống nhất, để ai (người hay agent) cũng làm việc giống nhau

- **AC1.1**: Given máy đã cài Docker, Git, mise. When chạy `mise install && make tools`. Then có đúng phiên bản Go 1.23, Node 22, Python 3.12, uv và các tool sinh code, không phải cài tay thêm gì. Chạy được trên Windows (WSL2 hoặc Git Bash) và macOS/Linux.
- **AC1.2**: Given repo vừa clone. When chạy `make check`. Then lint, typecheck, test của `web/`, `api/`, `workers/` và kiểm gen-drift (AC2.2) đều chạy và xanh.
- **AC1.3**: Given commit message không theo Conventional Commits, hoặc staged file chứa chuỗi giống secret. When `git commit`. Then hook pre-commit (lefthook) chặn và báo lý do. Hook pre-push chạy `make check` cho khu vực có thay đổi.
- **AC1.5**: Given thư mục `.claude/` từ agent-kit. When chạy `claude --agent orchestrator` rồi `/task-status`. Then orchestrator đọc được `docs/plans/` và trả bảng trạng thái, không lỗi. Gate của orchestrator dùng `make check` local thay cho CI.
- **AC1.6**: Repo có `docs/adr/0001-*.md` ghi các quyết định đã chốt trong tab Kiến trúc, `docs/adr/0002-mtls-phase-2.md` và `docs/adr/0003-local-first-devops-deferred.md` (ghi lý do hoãn DevOps và điều kiện để làm lại).

### 3.2 Contracts và sinh code: là Dev, tôi muốn sinh type TS, Go, Python từ một nguồn, để ba ngôn ngữ không lệch nhau

- **AC2.1**: Given `contracts/openapi.yaml`, `internal-api.yaml`, `jobs/*.schema.json`, `events/*.schema.json`, `errors.yaml`. When `make gen`. Then sinh đủ code vào `web/lib/api/gen/`, `web/lib/sse/gen/`, `api/gen/`, `workers/rag_worker/contracts/`, `workers/rag_worker/clients/gen/`, và hằng số mã lỗi cho cả ba ngôn ngữ.
- **AC2.2**: Given sửa `contracts/` nhưng không commit code sinh ra. When chạy `make check-gen` (nằm trong `make check` và hook pre-push). Then lệnh fail (`make gen && git diff --exit-code`).
- **AC2.3**: Given một mã lỗi mới trong `errors.yaml`. When `make gen`. Then có key i18n vi/en tương ứng trong `web/messages/{vi,en}/errors.json`.
- **AC2.4**: `openapi.yaml` có sẵn các thành phần dùng chung: `Problem` (RFC 9457), `Cursor`, `Limit`, `IdempotencyKey`, security `bearer`, extension `x-permission` cho mỗi operation.

### 3.3 Khung Go API: là Dev, tôi muốn khung API có sẵn config, log, lỗi, middleware, health, để domain mới chỉ viết nghiệp vụ

- **AC3.1**: Given thiếu hoặc sai biến cấu hình bắt buộc. When khởi động `app api`. Then process thoát mã 1 với thông báo nêu đúng biến sai, và không in giá trị secret.
- **AC3.2**: Given stack đang chạy. When `GET /healthz`. Then trả 200 mà không kiểm phụ thuộc. When `GET /readyz`. Then trả 200 nếu Postgres, Redis, Qdrant đều phản hồi trong 1 giây, ngược lại trả 503 kèm tên phụ thuộc lỗi.
- **AC3.3**: Given một request bất kỳ. Then response có header `X-Request-ID`, và log access JSON có `request_id`, `trace_id`, route pattern (không phải URL thật), status, thời gian.
- **AC3.4**: Given handler panic hoặc lỗi không phân loại. Then client nhận 500 dạng problem+json với `code: INTERNAL` và `requestId`, không lộ stack hay SQL. Log ghi đủ stack.
- **AC3.5**: Given body lớn hơn giới hạn (mặc định 1 MB) hoặc JSON có field lạ. Then trả lần lượt 413 và 400 problem+json.
- **AC3.6**: Given một route mới đăng ký mà không khai báo quyền. When `go test ./...`. Then test `TestAllRoutesDeclarePermission` fail và nêu tên route.
- **AC3.7**: Given đang có request chạy dở và SSE đang mở. When container `api` nhận SIGTERM (`docker compose restart api`). Then:
  - `/readyz` chuyển 503;
  - request đang chạy hoàn tất (trong tối đa 25 giây);
  - client SSE nhận event `reconnect`, tự nối lại khi `api` lên lại và không mất sự kiện (dùng `Last-Event-ID`, AC12.5);
  - không request nào bị cắt giữa chừng.
- **AC3.8**: `GET /api/v1/system/info` trả `version`, `commit` và có đủ handler, service, routes, test. Đây là domain mẫu cho be-agent.

### 3.4 Khung Python worker: là Dev, tôi muốn runtime hàng đợi có sẵn retry, DLQ, trace, để stage mới chỉ viết hàm `run`

- **AC4.1**: Given job hợp lệ trên `jobs:echo`. When worker nhận. Then stage chạy, kết quả được ghi, job bước sau (nếu có) được publish **trước** khi `XACK`.
- **AC4.2**: Given stage ném `RetryableError`. Then job được thử lại với backoff `2^attempt × 5s` + jitter, tối đa 5 lần. Hết lượt thì vào `jobs:echo:dlq`, trạng thái nghiệp vụ là `Failed` kèm mã lỗi. Ở dev, hệ số backoff cấu hình được (`WORKER_BACKOFF_BASE`) để test chạy nhanh.
- **AC4.3**: Given job sai schema hoặc stage ném `PermanentError`. Then job vào DLQ ngay, không retry.
- **AC4.4**: Given worker bị kill (`docker kill`) khi đang xử lý. Then sau tối đa `visibility_timeout` + 30 giây, consumer khác (hoặc chính worker sau khi khởi động lại) nhận lại job qua `XAUTOCLAIM`, và kết quả cuối không bị trùng (stage idempotent).
- **AC4.5**: Given worker nhận SIGTERM. Then worker ngừng nhận job mới, làm nốt job đang chạy trong tối đa 25 giây và không ack job chưa xong.
- **AC4.6**: Given job mang `traceparent`. Then span của worker nằm cùng trace với request API đã tạo job.
- **AC4.7**: Worker có heartbeat `worker:<stage>:<id>` (TTL 45 giây) và metric `rag_worker_jobs_total`, `rag_worker_job_duration_seconds`, `rag_worker_queue_lag_seconds` (endpoint `/metrics`).
- **AC4.8**: Worker không chứa API key của provider. Mọi lời gọi model đi qua `GATEWAY_URL` với `X-Internal-Token`.

### 3.5 Model gateway tối thiểu: là Dev, tôi muốn worker gọi được model qua Go ngay từ M0, để đường đi dữ liệu ra ngoài đúng kiến trúc từ đầu

- **AC5.1**: Given worker gọi `POST /internal/v1/embed` với token đúng. Then Go chuyển tới provider đã cấu hình (ở dev là `mock-llm`) **qua Squid :3128**, trả vector và ghi số token, độ trễ, `trace_id`.
- **AC5.2**: Given gọi `/internal/*` thiếu token, sai token, hoặc đi qua Nginx từ bên ngoài. Then lần lượt trả 401, 401, 404. Nginx không chuyển `/internal`.
- **AC5.3**: Given provider trả 429 hoặc 5xx (`X-Mock-Fail`). Then gateway retry theo backoff có jitter và tôn trọng `Retry-After`. Hết lượt thì trả lỗi đã phân loại để worker coi là `RetryableError`.
- **AC5.4**: Provider ở M0 cấu hình bằng file seed cho dev. Màn Cấu hình Model và mã hóa API key thuộc M3.

### 3.6 Khung Next.js: là Dev, tôi muốn vỏ app đúng UI canvas và lớp gọi API có sẵn, để mỗi tính năng chỉ thêm trang

- **AC6.1**: Given đăng nhập dev. When mở `/system`. Then thấy vỏ app đúng UI canvas:
  - sidebar 256px, thu gọn còn 56px;
  - header 56px có breadcrumb;
  - avatar góc dưới trái mở menu;
  - menu mở Settings dạng Dialog;
  - trang cao `h-dvh`, nội dung cuộn bên trong.
- **AC6.2**: Trang `/system` hiển thị `version`, `commit` từ API, và có đủ 4 trạng thái loading, empty, error, không có quyền. Mỗi trạng thái có test Vitest.
- **AC6.3**: Given access token hết hạn. When gọi API. Then client tự refresh **một lần** (nhiều request đồng thời chỉ refresh một lần) rồi gọi lại. Refresh thất bại thì chuyển `/login?next=…`.
- **AC6.4**: Given API trả problem+json. Then UI hiện thông điệp theo `code` bằng ngôn ngữ đang chọn, kèm `requestId`.
- **AC6.5**: Chuyển vi/en và light/dark không reload trang. Không có chuỗi hardcode (lint `i18next/no-literal-string` hoặc tương đương).
- **AC6.6**: Response có CSP `default-src 'self'` (không `unsafe-eval` ở build production; `next dev` được phép nới lỏng), `frame-ancestors 'none'`, `X-Content-Type-Options: nosniff`, `Referrer-Policy`.

### 3.7 Dữ liệu và migration: là Dev, tôi muốn schema và kho khởi tạo tự động, có phiên bản, quyền tối thiểu

- **AC7.1**: Given DB trống. When `make migrate`. Then:
  - chạy migration nền (`pgcrypto`, `citext`, trigger `set_updated_at`, `audit_logs` partition tháng, `idempotency_keys`, `echo_jobs`, `feature_flags`);
  - chạy `down` rồi `up` lại đều sạch.
- **AC7.2**: Given Postgres. Then:
  - có 3 user: `rag_owner` (DDL, chỉ container migrate dùng), `rag_app` (DML), `rag_readonly`;
  - `rag_app` không có quyền `UPDATE`/`DELETE` trên `audit_logs`.
- **AC7.3**: Given MinIO trống. When bước `bootstrap` chạy (tự chạy trong `make dev`). Then:
  - có 4 bucket `rag-raw`, `rag-parsed`, `rag-crawl`, `rag-export`;
  - `rag-raw` bật versioning;
  - lifecycle: `rag-crawl` xóa sau 90 ngày, `rag-export` sau 7 ngày;
  - chặn truy cập public;
  - user ứng dụng chỉ có quyền trên 4 bucket này.

  Chạy lại bootstrap không lỗi và không đổi gì.
- **AC7.4**: Có hàm `EnsureCollection(model)` tạo collection Qdrant gồm dense + sparse (IDF modifier) và payload index `kb_id`, `doc_id`, có test tích hợp. M0 chưa gọi hàm này từ UI.
- **AC7.5**: Given chưa có System Admin. When `docker compose run --rm api admin create --email …`, mật khẩu nhập qua stdin. Then tạo được System Admin đầu tiên. Gọi lại khi đã có System Admin thì bị từ chối. Mật khẩu không xuất hiện trong log hay tham số dòng lệnh.
- **AC7.6**: Scheduler tạo trước partition tháng sau cho các bảng partition, và xóa partition quá hạn theo bảng "giữ bao lâu" trong tab Kiến trúc mục 4.
- **AC7.7**: `make reset` xóa sạch volume dữ liệu local rồi chạy lại `migrate` + `bootstrap` + `seed` (hỏi xác nhận trước khi xóa).

### 3.8 Môi trường dev local: là Dev, tôi muốn chạy toàn bộ stack trên một máy, không tốn tiền API, sửa code là thấy ngay

- **AC8.1**: Given `make dev`. Then lên các thành phần sau, mỗi thành phần có healthcheck xanh:
  - nginx (`https://rag.localhost`, cert `mkcert`);
  - web (`next dev`), api (air), scheduler;
  - worker-echo (watchfiles);
  - postgres, redis, qdrant, minio;
  - squid;
  - mock-llm, mailpit.

  Thành phần tùy chọn, bật bằng profile:
  - `make dev-obs` (profile `obs`): otel-collector, prometheus, grafana, loki, tempo;
  - clamd (profile `scan`): chưa cần ở M0, bật khi làm M4 vì tốn ~1 GB RAM.
- **AC8.2**: Given sửa một file Go, Python hoặc TSX. Then service tương ứng tự nạp lại trong ≤ 5 giây, không cần restart stack.
- **AC8.3**: `mock-llm` có:
  - `/v1/chat/completions` (cả `stream` và `tool_calls` theo fixture);
  - `/v1/embeddings` (feature hashing, xác định, số chiều cấu hình được);
  - `/v1/rerank`;
  - header `X-Mock-Fail` và `X-Mock-Latency-Ms`.

  Cùng input luôn cho cùng output.
- **AC8.4**: `make seed` tạo:
  - tài khoản dev, một workspace, một KB mẫu;
  - provider trỏ vào `mock-llm`.

  Mật khẩu dev chỉ nằm trong `.env.example`, kèm nhãn "chỉ dùng cho dev".
- **AC8.5**: Data service chỉ mở cổng trên `127.0.0.1`. Squid ở dev vẫn bắt buộc: gọi provider không qua Squid thì thất bại.
- **AC8.6**: `make secrets-dev` sinh `jwt_private_key`, `master_key`, `internal_token` vào `deploy/secrets/` (đã `.gitignore`), mount vào container dạng Docker secret. Không có secret trong env, image hay git.
- **AC8.7**: `make help` liệt kê mọi lệnh (`dev`, `dev-obs`, `down`, `reset`, `logs`, `migrate`, `seed`, `gen`, `check`, `sec`, `smoke`, `token`) kèm mô tả một dòng.

### 3.10 Quan sát (local): là Dev, tôi muốn đi từ một request lỗi tới đúng trace và log trong vài cú click

- **AC10.1**: Given chạy `make dev-obs` và một echo job. When mở Grafana (`http://127.0.0.1:3001`) → Tempo. Then thấy một trace gồm span nginx → api handler → service → Redis XADD → worker stage → gateway → mock-llm. Từ span nhảy được sang log Loki cùng `trace_id`.
- **AC10.4 (local)**: Ở dev sampling 100%. Khi không bật profile `obs`, API và worker vẫn chạy bình thường (exporter OTLP lỗi không làm hỏng request).

### 3.11 Bảo mật nền (local): là Dev, tôi muốn base an toàn mặc định ngay trên máy dev, để không phải vá khi lên server

- **AC11.1 (local)**: Mạng:
  - chỉ Nginx mở 443/80 ra host;
  - data service chỉ bind `127.0.0.1`;
  - ra Internet chỉ qua Squid: :3128 chỉ domain provider, :3129 cho crawl và chặn dải private/link-local.
- **AC11.2**: Container:
  - user non-root;
  - `cap_drop: [ALL]`, `no-new-privileges`;
  - Dockerfile cho `api` (distroless), `worker`, `web` build được bằng `make images` (chỉ build local, chưa đẩy registry).
- **AC11.4**: `make sec` (gitleaks, semgrep, govulncheck, pip-audit, npm audit, trivy fs) chạy được trên local và xanh trước khi mở PR.
- **AC11.5 (local)**: Nginx:
  - TLS 1.2+ với cert `mkcert`;
  - `client_max_body_size 2m` cho `/api`;
  - `proxy_buffering off` cho SSE;
  - vhost `s3.rag.localhost` cho presigned URL;
  - không chuyển `/internal`.
- **AC11.6**: Middleware audit ghi `audit_logs` cho hành động nhạy cảm (ở M0 là tạo System Admin và chạy echo job), gồm: ai, việc gì, khi nào, IP, kết quả.

### 3.12 Walking skeleton "echo job": là Owner, tôi muốn thấy một luồng đi xuyên mọi tầng trước khi làm tính năng

- **AC12.1**:
  - Given System Admin ở `/system`.
  - When bấm "Chạy job thử" (gửi `Idempotency-Key`).
  - Then API trả 202 kèm `id`, ghi `echo_jobs` (`Queued`) và `audit_logs`, rồi `XADD jobs:echo`.
  - UI mở SSE và lần lượt thấy `Queued → Running → Done` cùng số token mà worker nhận từ `mock-llm` qua gateway.
- **AC12.2**: Given bấm hai lần với cùng `Idempotency-Key` trong 24 giờ. Then chỉ có một job; lần hai trả lại response của lần đầu.
- **AC12.3**: Given user không phải System Admin. Then không thấy nút. Gọi API trực tiếp thì nhận 403 problem+json.
- **AC12.4**: Given `X-Mock-Fail: 500` bật cho job. Then UI thấy `Retrying` (kèm số lần thử) rồi `Failed` kèm thông điệp theo mã lỗi; DLQ có job.
- **AC12.5**: Given SSE rớt giữa chừng. Then client nối lại với `Last-Event-ID` và không mất sự kiện.
- **AC12.6**: `make smoke` chạy `/readyz` + echo job e2e trên stack local và trả mã 0 khi thành công. Đây là bài kiểm cuối cùng của M0.

## 4. Giao diện
- Chỉ có trang `/system`, theo `Main.dc.html` (vỏ app) và `Settings.dc.html` (Dialog).
- Trong Settings, M0 chỉ dựng **khung** Dialog với điều hướng trái: các mục hiện "Sắp có" và bị vô hiệu. Nội dung từng mục thuộc M10.
- Quyền:
  - System Admin: thấy `/system` và nút "Chạy job thử".
  - Các vai trò khác: thấy `/system` ở chế độ chỉ đọc (version, commit). Nút bị ẩn.
- Trạng thái: loading (skeleton), empty (chưa chạy job nào), error (problem+json), không có quyền.

### 4.1 Xác thực tạm cho M0
M1 chưa có. Vì vậy M0 dùng **dev auth**: middleware `Authenticate` nhận JWT ký bằng `jwt_private_key`, token phát bởi `app dev-token --role system_admin|viewer` (gói trong `make token ROLE=…`).
- Lệnh này chỉ có khi `APP_ENV=dev`; build production không có.
- M1 thay nguồn phát token nhưng giữ nguyên middleware và claim.

## 5. Thay đổi API và contracts

| Method | Path | Quyền | Request | Response | Mã lỗi |
| --- | --- | --- | --- | --- | --- |
| GET | `/healthz` | công khai, ngoài `/api` | — | 200 | — |
| GET | `/readyz` | công khai, ngoài `/api` | — | 200 / 503 `{failing:[…]}` | — |
| GET | `/api/v1/system/info` | `authenticated` | — | `{version, commit}` | `UNAUTHENTICATED` |
| POST | `/api/v1/system/echo-jobs` | `system_admin` | header `Idempotency-Key`; `{message, failMode?}` | 202 `{id, status}` | `FORBIDDEN`, `VALIDATION_FAILED`, `IDEMPOTENCY_CONFLICT` |
| GET | `/api/v1/system/echo-jobs/{id}` | `system_admin` | — | `EchoJob` | `ECHO_JOB_NOT_FOUND` |
| GET | `/api/v1/system/echo-jobs/{id}/events` | `system_admin` | header `Last-Event-ID?` | SSE `echo.status` | như trên |
| POST | `/internal/v1/embed` | `X-Internal-Token` | `{model, inputs[]}` | `{vectors[][], usage}` | `UNAUTHORIZED_INTERNAL`, `PROVIDER_RATE_LIMITED`, `PROVIDER_UNAVAILABLE` |
| POST | `/internal/v1/chat` | `X-Internal-Token` | khung, chưa dùng ở M0 | — | — |

- **Job mới:** `contracts/jobs/envelope.schema.json` (`jobId`, `type`, `schemaVersion`, `attempt`, `traceparent`, `payload`) và `echo.schema.json`.
- **Event mới:** `contracts/events/echo-status.schema.json` (`id`, `status ∈ Queued|Running|Retrying|Done|Failed`, `attempt`, `tokens?`, `errorCode?`, `at`).
- **Mã lỗi mới trong `errors.yaml`:** `INTERNAL`, `UNAUTHENTICATED`, `FORBIDDEN`, `VALIDATION_FAILED`, `PAYLOAD_TOO_LARGE`, `IDEMPOTENCY_CONFLICT`, `ECHO_JOB_NOT_FOUND`, `UNAUTHORIZED_INTERNAL`, `PROVIDER_RATE_LIMITED`, `PROVIDER_UNAVAILABLE`.

## 6. Thay đổi dữ liệu
- Migration nền `20261012000000_baseline`:
  - extension `pgcrypto`, `citext`;
  - hàm `set_updated_at()`;
  - `audit_logs` (partition tháng, giữ 2 năm);
  - `idempotency_keys` (hết hạn 24 giờ);
  - `feature_flags` (`key`, `workspace_id?`, `enabled`).
- `echo_jobs` (`id`, `message`, `status`, `attempt`, `tokens`, `error_code`, `created_by`, timestamps).
  - Bảng này là dữ liệu kỹ thuật, có thể xóa sau Phase 1.
- Ba user Postgres và quyền như AC7.2. Không có dữ liệu cũ cần chuyển đổi.
- Quy ước schema theo tab Base dự án mục 8; mọi module sau áp dụng.

## 7. Yêu cầu phi chức năng

| Mục | Yêu cầu |
| --- | --- |
| Hiệu năng | `/system/info` p95 < 200 ms (PRD 7.1 "API CRUD"); echo job từ bấm tới `Done` < 5 giây với mock-llm |
| Thời gian | `make dev` lần đầu ≤ 10 phút; `make check` ≤ 10 phút; hot reload ≤ 5 giây |
| Tài nguyên | Stack mặc định (không `obs`, không `scan`) chạy được trên máy 16 GB RAM, Docker Desktop cấp ≤ 8 GB |
| Bảo mật | Theo PRD 7.2 và AC11 (local). Ở M0, **không dữ liệu nào ra LLM provider thật**: dev chỉ dùng `mock-llm` |
| Quan sát | Theo AC10.1; log JSON có `trace_id` (PRD 7.4) |
| Vận hành | Migration tương thích ngược ngay từ đầu (để khi có staging không phải sửa lại) |
| i18n, a11y | vi/en; WCAG 2.2 AA cho vỏ app (focus ring, bàn phím, `aria-label` cho nút chỉ có icon) |
| Tài liệu | README "từ máy trắng tới chạy được" (Windows + macOS/Linux); ADR 0001, 0002, 0003 |

## 8. Phát hành
- Không cần chuẩn bị hạ tầng ngoài: chỉ cần máy dev có Docker, Git, mise, `mkcert`.
- Repo vẫn đẩy lên GitHub để làm PR, nhưng chưa có Actions; gate là `make check` + `make sec` chạy local (orchestrator chạy trước khi mở PR).
- Không có feature flag nào cho người dùng cuối. Bảng `feature_flags` được dựng sẵn cho các module sau.
- Thứ tự gợi ý cho `/plan-spec`: tab Base dự án mục 2 và 13, **bỏ** task CI/CD (M0-07) và phần production của M0-08; giữ Dockerfile và compose local. Orchestrator có thể chia lại.

## 9. Câu hỏi mở
1. Dev chạy trên Windows: dùng WSL2 (khuyến nghị, hot reload nhanh hơn do file watch) hay Git Bash + Docker Desktop? Ảnh hưởng README và `make`.
2. Các câu hỏi về GitHub/GitLab, Harbor, staging, cert nội bộ, kênh alert, SMTP, chính sách giữ log → chuyển sang lúc làm phần DevOps (mục 10).

## 10. Hoãn lại: DevOps

Các AC dưới đây giữ nguyên mã gốc để làm tiếp khi có hạ tầng. Không phải phạm vi của M0 lần này; agent **không** được làm.

| AC | Nội dung | Điều kiện để làm lại |
| --- | --- | --- |
| AC1.4 | Ruleset nhánh `main` (PR bắt buộc, 1 approval, CODEOWNERS, status `gate`, chặn force push, squash only) | Chốt GitHub hay GitLab |
| AC9.1 | CI theo khu vực thay đổi, job `gate`, p95 ≤ 15 phút | Như trên |
| AC9.2 | Build 3 image `sha-<commit>`, `trivy image`, SBOM `syft`, ký `cosign`, đẩy Harbor | Có Harbor |
| AC9.3 | Staging tự `migrate` → `pull` → `up -d`, smoke test, tự rollback | Có máy staging + runner self-hosted |
| AC9.4 | Tag `vX.Y.Z` re-tag image, CHANGELOG, duyệt deploy production | Có môi trường production |
| AC9.5 | Pin action theo SHA, pin image theo digest, `renovate.json` | Có CI |
| AC9.6 | e2e và quét dependency hằng đêm, tự mở issue | Có CI |
| AC10.2 | Dashboard dạng code (API RED, worker/queue, LLM, hạ tầng) | Có staging; có thể làm sớm nếu rảnh |
| AC10.3 | Alert + runbook (5xx, TTFT, queue lag, DLQ, provider lỗi, chi phí, ổ đĩa, backup) | Chốt kênh alert và người trực |
| AC10.4 | Sampling 100% lỗi / 10% thường, giữ log 30 ngày | Có staging |
| AC11.1 (phần server) | `DATA_BIND_IP`, tách 2 máy app/data | Có máy chủ |
| AC11.2 (phần server) | `read_only: true` cho api/web, giới hạn CPU/RAM | Có staging |
| AC11.3 (phần vận hành) | Quyền file secret 600 trên server, runbook xoay `internal_token` và `master_key` | Có staging |
| AC11.5 (phần server) | HSTS, cert TLS nội bộ, DNS `rag-staging.<domain>` | Chốt CA nội bộ |
| AC11.7 | Backup `pg_dump` + WAL-G, snapshot Qdrant, `mc mirror`; khôi phục thử, RPO 24h / RTO 4h | Có máy chủ |
| — | Compose production theo profile `app`/`data`/`obs` trên 2 máy, tài liệu cài đặt | Có máy chủ |
| — | MinIO server-side encryption (cần KMS) | Có máy chủ |
