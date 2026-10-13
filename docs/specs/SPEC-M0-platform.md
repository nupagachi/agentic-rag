---
id: SPEC-M0-platform
title: Platform — khung monorepo tối thiểu chạy được trên local
owner: "@<owner>"            # thay bằng GitHub username
status: draft
scope_note: "Chỉ dựng khung tối thiểu để 3 ứng dụng khởi động được và có make check. Cấu trúc còn lại được bổ sung dần theo từng module, khi module đó cần (mục 3). Không CI/CD, không hạ tầng server. Bản đầy đủ trước đây lưu ở docs/specs/_archive/SPEC-M0-platform-full.md."
prd_refs: ["7.4 Vận hành", "8 Kiến trúc", "8.1 Tech stack"]
p0_module: M0
p0_tasks: [M0-01, M0-02, M0-03]   # tối thiểu; phần còn lại của M0-02..M0-08 rải vào module dùng tới (mục 3)
docs_refs:
  - "docs/Kiến trúc.md (mục 2 tech stack, mục 3 cấu trúc code)"
  - "docs/Phân rã P0.md (module M0)"
  - "docs/Hướng dẫn setup & workflow agent.md"
---

# SPEC-M0: Platform (khung tối thiểu, local)

## 1. Vấn đề và mục tiêu

**Vấn đề.**
- Repo mới chỉ có `docs/` và `.claude/`. Chưa có code, môi trường chạy hay bộ lệnh chung.
- Workflow agent (orchestrator, `make check-<area>`) chưa chạy được vì chưa có repo và lệnh kiểm.
- Dựng sẵn toàn bộ cấu trúc từ đầu tốn công và dễ sai, vì nhiều quyết định chỉ rõ khi làm tới tính năng thật.

**Mục tiêu đo được.**
1. Máy dev đã cài Docker Desktop, Git, `make` và Go/Node/Python (hoặc `mise`) clone repo, làm theo README và chạy được khung trong **≤ 10 phút**.
2. Ba ứng dụng `web`, `api`, `workers` đều **khởi động được** trên local.
3. `make check` (lint, typecheck, test cho cả 3 ứng dụng) chạy xanh trong **≤ 3 phút**. Đây là cổng kiểm trước khi mở PR.

## 2. Nguyên tắc: cấu trúc tăng dần

- M0 chỉ tạo những gì **mọi** module sau đều cần ngay: thư mục 3 ứng dụng, bộ lệnh `make`, lint/test, Postgres + Redis local.
- Mọi thư mục, thư viện, service Docker, file contract còn lại do **module đầu tiên cần đến nó** thêm vào, trong cùng PR với tính năng đó, và chỉ ở mức tính năng đó dùng.
- Đích cuối vẫn là cấu trúc trong `docs/Kiến trúc.md` mục 3. Module thêm cấu trúc phải đặt đúng vị trí và tên đã ghi ở đó, không tự đặt chỗ khác.
- Khi orchestrator lập plan cho một module, task đầu tiên của module (thường ở wave 0) gồm luôn phần cấu trúc mới cần thêm theo bảng dưới.

**Bảng gợi ý: module nào thêm cấu trúc gì** (không bắt buộc đúng từng dòng; orchestrator chỉnh khi lập plan)

| Cấu trúc | Thêm ở | Ghi chú |
| --- | --- | --- |
| `contracts/openapi.yaml`, `contracts/errors.yaml`, lệnh `make gen`, `check-gen`, client `web/lib/api/` | M1 (API đầu tiên) | Theo skill `contracts-first`. Từ đây task sửa contracts nằm ở wave 0 |
| `golang-migrate`, `api/migrations/`, `make migrate`, `sqlc` + `api/queries/` | M1 (bảng `users`, `sessions`) | Migration đầu tiên gồm luôn `pgcrypto`, `set_updated_at()` |
| `internal/auth`, `internal/authz`, trang `(auth)/`, Docker secret cho JWT key | M1 | |
| `internal/workspace`, trang KB | M2 | |
| `internal/llm/*`, `/internal/v1/*`, `contracts/internal-api.yaml`, `mock-llm` trong compose, `workers/rag_worker/clients/gateway.py` | M3 | Kèm Squid nếu gọi provider thật |
| `rag_worker/runtime/` (consumer, retry, DLQ), `contracts/jobs/`, `contracts/events/`, SSE | M4 (M4-02) | |
| MinIO + bucket, Qdrant, ClamAV trong compose; `contracts/bm25/`, `testdata/` | M4 | |
| `internal/retrieval`, `internal/agent` | M5 | |
| `internal/chat`, trang chat | M6 | |
| `internal/trace`, `internal/inspector`, OpenTelemetry | M7 (hoặc sớm hơn nếu cần gỡ lỗi) | |
| `internal/eval`, `rag_worker/eval/` | M8 | |
| `internal/admin`, `audit_logs`, quota | M9 | |
| `(app)/settings/` | M10 (hoặc M1 nếu cần trang hồ sơ sớm) | |

## 3. Ngoài phạm vi
- Mọi API nghiệp vụ, contracts, migration, luồng job, SSE, gateway.
- Mọi cấu trúc trong bảng mục 2 (do module tương ứng thêm).
- Toàn bộ DevOps và hạ tầng (mục 10): CI/CD, Dockerfile production, Nginx, TLS, Squid, Docker secrets, observability stack, backup.

## 4. User story và acceptance criteria

Vai trò: **Dev** (kỹ sư hoặc coding agent), **Owner** (người phụ trách dự án).

### 4.1 Repo và bộ lệnh: là Dev, tôi muốn một monorepo có lệnh thống nhất

- **AC1.1**: Repo có cấu trúc tối thiểu:
  ```
  agentic-rag/
  ├── web/          # Next.js 15 + TS strict
  ├── api/          # Go 1.27 (mới nhất): cmd/app/main.go, internal/platform/
  ├── workers/      # Python 3.12 + uv: rag_worker/, tests/
  ├── deploy/       # docker-compose.dev.yml, .env.example
  ├── docs/adr/
  ├── Makefile
  ├── mise.toml     # pin Go, Node, Python, uv
  ├── .editorconfig
  └── README.md
  ```
  Không tạo trước thư mục nào khác.
- **AC1.2**: Given máy có `mise`. When `mise install`. Then có đúng Go 1.27.x (bản ổn định mới nhất, hiện 1.27.2), Node 22, Python 3.12, uv. README ghi cách cài tay nếu không dùng mise.
- **AC1.3**: Given repo vừa clone, đã cài tool. When `make check`. Then chạy `check-web`, `check-api`, `check-workers`; tất cả xanh. Mỗi `check-<area>` chạy được riêng:
  - `check-web`: `eslint`, `tsc --noEmit`, `vitest run`.
  - `check-api`: `gofmt` (fail nếu chưa format), `go vet`, `golangci-lint run` (cấu hình tối thiểu), `go test ./...`.
  - `check-workers`: `ruff check`, `ruff format --check`, `mypy --strict rag_worker`, `pytest`.
- **AC1.4**: `make help` liệt kê mọi lệnh kèm mô tả một dòng: `env`, `up`, `down`, `reset`, `dev-api`, `dev-worker`, `dev-web`, `check`, `check-<area>`, `sec`. Module sau thêm lệnh thì thêm mô tả vào đây.
- **AC1.5**: Makefile chạy được trên macOS/Linux và Windows (Git Bash hoặc WSL2). README ghi cách cài `make` trên Windows.
- **AC1.6**: Given `.claude/` hiện có. When `claude --agent orchestrator` rồi `/task-status`. Then orchestrator đọc được `docs/plans/` và trả bảng trạng thái, không lỗi.
- **AC1.7**: Có `docs/adr/0001-incremental-structure.md` ghi nguyên tắc mục 2, lý do M0 chỉ chạy local và danh sách hoãn ở mục 10.

### 4.2 Khung Go API

- **AC2.1**: `api/cmd/app/main.go` với lệnh con `app api`. Router `chi`, kết nối Postgres (`pgx` pool) và Redis (`go-redis`) đặt trong `internal/platform/`.
- **AC2.2**: Given thiếu hoặc sai biến cấu hình bắt buộc. When `app api`. Then thoát mã 1, nêu đúng tên biến sai, không in giá trị secret.
- **AC2.3**: `GET /healthz` trả 200 không kiểm phụ thuộc. `GET /readyz` trả 200 nếu Postgres và Redis phản hồi trong 1 giây, ngược lại 503 `{failing: [...]}`. Đây là 2 route duy nhất ở M0.
- **AC2.4**: Log JSON bằng `slog`; middleware `X-Request-ID`, log access (`request_id`, `method`, route pattern, `status`, `duration_ms`), recover panic → 500 JSON không lộ stack. Có unit test cho từng middleware.
- **AC2.5**: Given SIGINT/SIGTERM. Then server ngừng nhận request mới, chờ request đang chạy tối đa 10 giây rồi thoát.
- **AC2.6**: Server listen `127.0.0.1:8080` (cấu hình được). Có test cho `/healthz` và `/readyz`.

### 4.3 Khung Python worker

- **AC3.1**: Package `rag_worker` quản lý bằng `uv` (`pyproject.toml`). `python -m rag_worker` đọc config (pydantic-settings), log JSON, in "ready" rồi chờ; thoát gọn khi SIGINT/SIGTERM.
- **AC3.2**: Thiếu biến cấu hình bắt buộc thì thoát mã 1, nêu tên biến, không in secret.
- **AC3.3**: Có ít nhất 1 test cho config. Chưa tạo `runtime/`, `stages/`, `clients/` (M3, M4 thêm).

### 4.4 Khung Next.js

- **AC4.1**: Next.js 15 App Router, TypeScript strict, Tailwind 4, shadcn/ui, next-intl (vi/en). `make dev-web` chạy tại `http://localhost:3000`. TanStack Query, react-hook-form thêm khi trang đầu tiên cần.
- **AC4.2**: Vỏ app trong `app/(app)/layout.tsx`: sidebar trái thu gọn được, header, vùng nội dung cuộn bên trong; nút đổi vi/en và sáng/tối (không reload trang). Menu chỉ có "Trang chủ".
- **AC4.3**: Trang chủ placeholder hiển thị tên ứng dụng và trạng thái `/healthz` của API (API tắt thì hiện "không kết nối được"). Có test Vitest cho 2 trạng thái.
- **AC4.4**: Không có chuỗi giao diện hardcode (lint rule tương đương `no-literal-string` cho `app/`, `components/`). Bám `docs/Agentic RAG UI.html` ở mức màu, font, khoảng cách.

### 4.5 Môi trường dev local

- **AC5.1**: `make up` chạy `deploy/docker-compose.dev.yml` chỉ gồm `postgres:16` và `redis:7.2`, có healthcheck; lệnh chờ healthy rồi mới trả về. Service khác do module cần tới thêm (mục 2).
- **AC5.2**: Cổng chỉ bind `127.0.0.1`. Image pin tag cụ thể (không `latest`).
- **AC5.3**: Ba ứng dụng chạy trên host, mỗi cái một terminal, có hot reload ≤ 5 giây: `make dev-api` (`air`), `make dev-worker` (`watchfiles`), `make dev-web` (`next dev`).
- **AC5.4**: Cấu hình qua `.env` ở gốc repo; `make env` copy từ `deploy/.env.example` nếu chưa có. `.env.example` chỉ chứa giá trị dev, ghi rõ "chỉ dùng cho dev". `.env` nằm trong `.gitignore`.
- **AC5.5**: `make reset` hỏi xác nhận, xóa volume dữ liệu local rồi chạy lại `up`.

### 4.6 Bảo mật nền

- **AC6.1**: Postgres, Redis và API chỉ bind `127.0.0.1`.
- **AC6.2**: `make sec` chạy được: `gitleaks detect`, `govulncheck ./...`, `pip-audit`, `npm audit --audit-level=high`; xanh trên code M0.
- **AC6.3**: Không log giá trị secret; config in ra lúc khởi động phải che secret.

### 4.7 Nghiệm thu cuối

- **AC7.1**: Trên máy sạch: `make env && make up`, rồi 3 terminal `make dev-api`, `make dev-worker`, `make dev-web`. Then `curl 127.0.0.1:8080/readyz` trả 200, worker log "ready", trang `http://localhost:3000` hiện API "đang chạy".
- **AC7.2**: `make check` và `make sec` xanh.

## 5. Giao diện
- Chỉ có vỏ app và trang chủ placeholder (AC4.2, AC4.3).

## 6. Thay đổi API, contracts, dữ liệu
- Không có. Chỉ `GET /healthz`, `GET /readyz`. Chưa có `contracts/`, chưa có migration (M1 thêm).

## 7. Yêu cầu phi chức năng

| Mục | Yêu cầu |
| --- | --- |
| Thời gian | Setup lần đầu ≤ 10 phút; `make check` ≤ 3 phút; hot reload ≤ 5 giây |
| Tài nguyên | `make up` chạy được khi Docker Desktop cấp ≤ 2 GB RAM |
| Bảo mật | Mục 4.6. Không gọi LLM provider nào |
| i18n, a11y | vi/en; focus ring, điều hướng bàn phím, `aria-label` cho nút chỉ có icon |
| Tài liệu | README "từ máy trắng tới chạy được" cho Windows và macOS/Linux; ADR 0001 |

## 8. Phát hành
- Repo đẩy lên GitHub để làm PR, chưa có Actions. Gate trước khi mở PR: `make check` (+ `make sec`), do orchestrator chạy.
- Gợi ý chia wave cho `/plan-spec`:
  - **Wave 0**: khung gốc (`Makefile`, `mise.toml`, `.editorconfig`, README, ADR, compose dev, `.env.example`).
  - **Wave 1** (song song): khung Go API; khung worker; khung Next.js + vỏ app.

## 9. Câu hỏi mở
1. Windows: Git Bash + Docker Desktop (mặc định) hay bắt buộc WSL2?
2. Ba ứng dụng chạy trên host (mặc định) hay trong compose?
3. Hook pre-commit (lefthook) ở M0? Mặc định: chưa làm.

## 10. Hoãn lại (không gắn module cụ thể)

Agent **không** làm trong M0. Chi tiết gốc ở `docs/specs/_archive/SPEC-M0-platform-full.md`.

| Hạng mục | Làm khi |
| --- | --- |
| CI/CD, Dockerfile production, registry, compose production 2 máy | Có runner / máy chủ |
| Nginx + TLS, Squid egress proxy | Trước khi gọi provider thật (M3) hoặc upload presigned (M4) |
| Prometheus, Grafana/Loki/Tempo | Khi có staging |
| Backup, alert, runbook | Có máy chủ |
