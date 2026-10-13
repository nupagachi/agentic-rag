# Plan SPEC-M0-platform: Khung monorepo tối thiểu chạy được trên local

SPEC: `docs/specs/SPEC-M0-platform.md` · Threat model: `threat-model.md` · Lập: 2026-10-13 · Duyệt bởi Owner: 2026-10-13

## Tóm tắt
Dựng khung tối thiểu: bộ lệnh `make` + compose dev (Postgres, Redis), khung Go API (`/healthz`, `/readyz`, middleware nền), khung Python worker (khởi động, config, log), khung Next.js (token, i18n, theme, vỏ app, trang chủ báo trạng thái API). Chia 7 task, 3 wave, 7 PR. Không có contracts/migration (M1 thêm theo nguyên tắc cấu trúc tăng dần). Dùng Go mới nhất (1.27.x) theo quyết định Owner. Rủi ro lớn nhất: repo chưa có commit/remote nên chưa mở PR được.

## Wave
| Wave | Task | Owner | Area | Size | Phụ thuộc | Trạng thái | PR |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 0 | M0-T01 Khung gốc: Makefile ủy quyền, mise, compose dev, README, ADR, `make sec` | be-agent | deploy | M | — | ready | |
| 1 | M0-T02 Khung Go API: config, log, pgx/redis, /healthz, /readyz, shutdown | be-agent | api | M | T01 | todo | |
| 1 | M0-T03 Khung Python worker: uv, config, log, signal, ping Redis | be-agent | workers | S | T01 | todo | |
| 1 | M0-T04 Khung Next.js: Next 15, Tailwind 4 token, shadcn, next-intl, theme, lint, Vitest | fe-agent | web | M | T01 | todo | |
| 2 | M0-T05 Middleware Go: request id, access log, recover, CORS | be-agent | api | S | T02 | todo | |
| 2 | M0-T06 Vỏ app + trang chủ trạng thái API | fe-agent | web | M | T04 | todo | |
| 2 | M0-T07 Test tích hợp: /readyz + worker với Postgres/Redis thật | test-agent | api, workers (thư mục test) | S | T02, T03 | todo | |

Kiểm `scope.paths` không giao nhau trong cùng wave:
- Wave 1: `api/**` (T02, trừ `httpx/`) · `workers/**` (T03) · `web/**` trừ `(app)/`, `app-shell/` (T04).
- Wave 2: `api/internal/platform/{httpx,server}/**` + `go.mod` (T05) · `web/app/(app)/**`, `components/app-shell/**` (T06) · `api/test/integration/**`, `workers/tests/integration/**`, target `test-integration` trong `api/Makefile`, `workers/Makefile` (T07).
- Root `Makefile` chỉ T01 sửa. Wave sau cắm vào qua `<app>/Makefile` (target `check`, `dev`, `sec`, tùy chọn `test-integration`).

## Đồ thị phụ thuộc
```
T01 ─┬─> T02 ─┬─> T05
     │        └─┐
     ├─> T03 ───┴─> T07
     └─> T04 ───> T06
```

## Truy vết AC SPEC → task
| SPEC AC | Task |
| --- | --- |
| AC1.1–1.5, 1.7, AC5.1, 5.2, 5.4, 5.5, AC6.1 (compose), AC6.2 | T01 |
| AC1.3 (`check-<area>`), AC5.3 | T02, T03, T04 (Makefile con) |
| AC2.1–2.3, 2.5, 2.6, AC6.3 (Go) | T02 |
| AC2.4 | T05 |
| AC3.1–3.3, AC6.3 (Python) | T03 |
| AC4.1, 4.4 | T04 |
| AC4.2, 4.3 | T06 |
| AC7.1 | T07 (phần tự động) + Owner nghiệm thu tay sau wave 2 |
| AC1.6 (`/task-status`) | Orchestrator tự kiểm sau wave 0 |
| AC7.2 | Cổng mỗi PR |

## Dependency mới cần Owner duyệt (duyệt plan = duyệt danh sách này)
- **Go:** `go-chi/chi/v5`, `jackc/pgx/v5`, `redis/go-redis/v9`, `caarlos0/env/v11` (hoặc tự đọc env), `joho/godotenv`, `go-chi/cors`. Tool: `air`, `golangci-lint`, `govulncheck`.
- **Python:** `pydantic-settings`, `redis`, `structlog` (hoặc stdlib); dev: `watchfiles`, `pytest`, `ruff`, `mypy`, `pip-audit`.
- **Web:** `next@15` (≥ 15.5.7), `react@19` (bản vá), Tailwind 4, shadcn (+ radix, cva, clsx, tailwind-merge, lucide-react), `next-intl`, `next-themes`, `geist`, ESLint (`eslint-config-next`, `typescript-eslint`, plugin no-literal-string), Vitest + Testing Library + jsdom.
- **Image:** `postgres:16.x`, `redis:7.2.x` (x ≥ 11).
- **Tool chung:** `mise`, `gitleaks`.

## Rủi ro và cách giảm
| Rủi ro | Ảnh hưởng | Cách giảm |
| --- | --- | --- |
| Thư viện Go chưa hỗ trợ Go 1.27 | Build lỗi ở T02 | Thư viện trong danh sách đều theo kịp bản Go mới; nếu lỗi, be-agent báo `SCOPE_CHANGE_NEEDED` thay vì hạ phiên bản |
| Repo chưa có commit, chưa có remote, máy chưa có `gh` | Không tạo được worktree/branch từ main, không mở PR | Owner làm bước chuẩn bị trước wave 0 (mục Câu hỏi mở 2) |
| Makefile chạy khác nhau giữa Git Bash và macOS/Linux | Dev Windows không chạy được lệnh | T01 AC10: chỉ dùng cú pháp make/bash chung; kiểm trên máy Windows của Owner |
| Hot reload trên Windows chậm (file watch qua Docker Desktop) | Không đạt ≤ 5 giây | Ứng dụng chạy trên host, chỉ data service trong Docker (đã chọn trong SPEC) |
| shadcn/Tailwind 4/Next 15 ra bản mới thay đổi cấu hình | fe-agent tốn vòng sửa | Pin phiên bản trong lockfile; T04 ghi rõ phiên bản chọn trong PR |
| Thư mục rỗng cũ (`contracts/bm25`, `deploy/nginx`…) gây hiểu nhầm | Lệch nguyên tắc "không tạo trước" | Git không theo dõi thư mục rỗng; Owner có thể xóa tay. Agent không thêm `.gitkeep` |

## Câu hỏi mở
1. ~~Go version~~ **Đã chốt (2026-10-13):** dùng Go mới nhất, hiện là 1.27.2. `go.mod` ghi `go 1.27`, `mise.toml` pin `1.27.2`. Đã cập nhật SPEC (AC1.1, AC1.2), `docs/Kiến trúc.md` mục 2 + Dockerfile mẫu, mô tả `be-agent`.
2. **Chuẩn bị git (bắt buộc trước wave 0)**: repo chưa có commit và remote; máy chưa cài `gh`. Owner cần:
   - tạo commit đầu tiên trên `main` gồm `.claude/`, `.gitignore`, `docs/` và push lên GitHub;
   - cài và đăng nhập `gh` (`winget install GitHub.cli` → `gh auth login`).
   Orchestrator không được commit/push lên `main`.
3. **Redis 7.2**: chấp nhận pin `7.2.x` (x ≥ 11)? Nếu nhánh 7.2 đã hết hỗ trợ lúc cài thì chuyển sang 7.4.
4. **`make sec`** thiếu semgrep, trivy (so với skill `security-standards`): chấp nhận cho M0, bổ sung khi có CI?
5. **e2e Playwright** không làm ở M0 (chỉ 1 trang placeholder, đã có test Vitest); bắt đầu từ M1. Đồng ý?
6. SPEC còn để `owner: "@<owner>"`; frontmatter `scope_note` ghi "mục 3" nhưng nguyên tắc nằm ở mục 2 (lỗi nhỏ, Owner sửa khi duyệt).

## Nhật ký
- 2026-10-13 Owner duyệt plan; câu 3–6 theo mặc định. Chế độ chạy: local, chưa PR (chưa có remote/gh).
- 2026-10-13 Owner chốt Go mới nhất (1.27.2) thay cho 1.23; cập nhật T01, T02.
- 2026-10-13 Lập plan. Threat model: 20 SR (7 High, 10 Medium, 3 Low), đã gắn vào task. Bỏ bước Explore vì repo chưa có code.
