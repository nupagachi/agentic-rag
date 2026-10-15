# Plan SPEC-M0-platform: Khung monorepo tối thiểu chạy được trên local

SPEC: `docs/specs/SPEC-M0-platform.md` · Threat model: `threat-model.md` · Lập: 2026-10-13 · Duyệt bởi Owner: 2026-10-13

## Tóm tắt
Dựng khung tối thiểu: bộ lệnh `make` + compose dev (Postgres, Redis), khung Go API (`/healthz`, `/readyz`, middleware nền), khung Python worker (khởi động, config, log), khung Next.js (token, i18n, theme, vỏ app, trang chủ báo trạng thái API). Chia 7 task, 3 wave, 7 PR. Không có contracts/migration (M1 thêm theo nguyên tắc cấu trúc tăng dần). Dùng Go mới nhất (1.27.x) theo quyết định Owner. Rủi ro lớn nhất: repo chưa có commit/remote nên chưa mở PR được.

## Wave
| Wave | Task | Owner | Area | Size | Phụ thuộc | Trạng thái | PR |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 0 | M0-T01 Khung gốc: Makefile ủy quyền, mise, compose dev, README, ADR, `make sec` | be-agent | deploy | M | — | merged | local ff 99e11af |
| 1 | M0-T02 Khung Go API: config, log, pgx/redis, /healthz, /readyz, shutdown | be-agent | api | M | T01 | merged | local ff 389df81 |
| 1 | M0-T03 Khung Python worker: uv, config, log, signal, ping Redis | be-agent | workers | S | T01 | merged | local ff 5149698 |
| 1 | M0-T04 Khung Next.js: Next 15, Tailwind 4 token, shadcn, next-intl, theme, lint, Vitest | fe-agent | web | M | T01 | merged | local ff d61b94a |
| 2 | M0-T05 Middleware Go: request id, access log, recover, CORS | be-agent | api | S | T02 | merged | local ff eb66026 |
| 2 | M0-T06 Vỏ app + trang chủ trạng thái API | fe-agent | web | M | T04 | blocked | |
| 2 | M0-T07 Test tích hợp: /readyz + worker với Postgres/Redis thật | test-agent | api, workers (thư mục test) | S | T02, T03 | merged | local ff 6ea6922 |

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
- 2026-10-14 M0-T07 sửa xong (2 commit: che mật khẩu trong thông điệp lỗi, 2 dạng mật khẩu, đo 1,5 s, TestMain dọn tạm, SIGINT Unix, t.Context). Hook chặn implementer rebase → orchestrator rebase; check-api + check-workers rc=0, test-integration chạy → ff main 6ea6922. M0-T06 sửa a11y xong, rebase lên main, check-web 71 test + sec rc=0; blocked chờ Owner duyệt ngoại lệ sửa web/i18n (commit đăng ký namespace).
- 2026-10-14 Review wave 2: cả 3 task APPROVE + PASS, không blocker/major. T05 sửa 1 lượt (recover abort khi response đã bắt đầu, host origin chữ thường, body 500 qua json.Marshal) → check + sec rc=0 → ff main eb66026. T06 gửi sửa a11y (contrast chữ lỗi, focus sau retry, khớp nav, useId). T07 gửi sửa (che mật khẩu trong thông điệp lỗi test, kiểm 2 dạng mật khẩu, đo 1,5 s ở test luôn chạy, xóa thư mục tạm, assert SIGINT trên Unix, t.Context) + rebase lên main. Follow-up đề xuất: token màu light mode < 4.5:1 (brand/success/destructive trên nền soft), thêm shadcn breadcrumb/card/alert/badge, lint build tag integration.
- 2026-10-14 M0-T06 code xong (4 commit, +1132, code ~540 dòng vượt size M): check-web 65 test + build, sec rc=0. SCOPE_CHANGE_NEEDED agent tự xử lý: commit 070f886 sửa 5 dòng web/i18n/{config,messages}.ts (forbidden) để đăng ký namespace shell/home → chờ Owner. Cả 3 task code xong → chạy 3 review + 3 security song song.
- 2026-10-14 M0-T05 xong phần mở rộng (3 commit: default CORS ở config, bỏ fallback httpx/server, gắn middleware vào `app api`). check-api + sec rc=0, đúng scope mới. Gộp thử T05 + T07: vet + test integration pass. Còn chờ T06.
- 2026-10-14 M0-T07 code xong (2 commit, +877, vượt size S): check-api + check-workers rc=0, đúng scope (chỉ thêm target test-integration). Test chạy với stack thật (Docker đã lên giữa chừng, env truyền qua biến từ .env.example): api 5/5, workers 3/3 PASS; nhánh skip có thông báo. Rủi ro ghép nối: shutdown_test dùng server.New/Serve, T05 đang sửa server → kiểm lại sau khi T05 xong. Trailer commit ghi Sonnet 5.5 (đúng model chạy agent).
- 2026-10-14 M0-T05 lượt 1: 4 middleware + chain (5 commit, +1115, phần lớn test; vượt size S), check-api + sec rc=0, đúng scope; CORS tự viết, không thêm go-chi/cors. Báo SCOPE_CHANGE_NEEDED: binary chưa gắn middleware (api/cmd forbidden). Owner duyệt mở rộng scope T05 sang api/cmd/app/** và config/** (default CORS origin đặt ở config, middleware rỗng = không cấp CORS). Gửi be-agent làm tiếp trên cùng branch.
- 2026-10-14 Owner duyệt wave 2, yêu cầu code xong cả 3 task rồi mới chạy review + security. Giao song song từ main e32b984: T05 (be), T06 (fe), T07 (test). Docker Desktop chưa chạy, chưa có file env local → T07 chỉ kiểm được nhánh skip; phần chạy stack thật Owner chạy tay.
- 2026-10-14 Wave 1 xong. M0-T04 vòng 1 (3 commit) và M0-T02 vòng 2 (6 commit): cổng check + sec rc=0, đúng scope; không re-review vì không có blocker/major. Rebase lên main rồi ff: T04 → d61b94a, T02 → 389df81. T05, T06, T07 → ready. Lưu ý môi trường: autocrlf làm api/go.mod, go.sum ra CRLF → `go mod tidy -diff` fail (đề xuất .gitattributes eol=lf); máy không có cgo nên chưa chạy -race.
- 2026-10-14 M0-T02 review lại (biên bản vòng 1 mất): review-agent APPROVE (6 minor gồm 2 ở WIP, 3 nit), security PASS (3 Low: WriteTimeout, URL có fragment lộ một phần mật khẩu, Secret trong field unexported; 4 Info). WIP secret.go lưu patch ở scratchpad. Gỡ worktree cũ T02 + T04 vì agent mới bị cô lập worktree, không vào được branch đang bị giữ (memory agent đã chuyển về .claude/agent-memory). Giao lại song song: fe-agent T04 vòng 1/2, be-agent T02 vòng 2/2. Chờ Owner: ngưỡng che mật khẩu ngắn, exit 1 khi quá timeout shutdown.
- 2026-10-14 M0-T04 review: review-agent APPROVE (3 minor, 2 nit), security PASS (1 Medium: dep thừa `cn`; 2 Low, 2 Info). Gửi fe-agent sửa vòng 1/2: gỡ `cn`, closeLabel cho Sheet, `make dev` chỉ export NEXT_PUBLIC_API_URL, `npm test -- --run`. Hoãn: cookie Secure (chờ Nginx), giữ exactOptionalPropertyTypes.
- 2026-10-14 Tiếp tục sau khi phiên bị ngắt. M0-T04 xong 4 commit (aac426f, +12.2k gồm lockfile); cổng check-web rc=0, sec rc=0, đúng scope → in-review. M0-T02 vòng sửa 1 dở (bb03ca2 module path + secret.go chưa commit); biên bản review vòng 1 mất → chạy lại review + security để lấy danh sách. make dùng tạm từ thư mục Temp (không có trong PATH).
- 2026-10-13 M0-T02 security PASS (3 Low, 4 Info). Gửi sửa vòng 1/2: module path github.com/nupagachi/agentic-rag/api + 3 Low + 5 minor + 1 nit.
- 2026-10-13 Owner cho phép tự merge mọi task xong review vào main. M0-T03 merged (ff 5149698). M0-T04 chấp nhận sec hai bước + deps radix-ui, tw-animate-css, @eslint/*. M0-T02 review: REQUEST_CHANGES (1 major module path tạm, 5 minor, 2 nit).
- 2026-10-13 M0-T03 sửa xong (5149698, tổng 154k token): Medium SR-7 đã sửa, orchestrator kiểm 3 URL sentinel → rc=1, JSON, không lộ; check-workers 35 pass. Không chạy re-review vì không có blocker/major. Status in-review.
- 2026-10-13 M0-T02 xong (3 commit, +2196 ≈ 975 code + 1180 test — vượt xa size M; 144k token, 71 tool call, ~20 phút). Cổng: check-api rc=0, sec no vulns, đúng scope. Nâng x/text v0.41.0 (vuln). Đang review.
- 2026-10-13 M0-T03 review: review-agent APPROVE (2 minor, 3 nit), security PASS (1 Medium: REDIS_URL query sai → traceback thô; 1 Low, 2 Info). Gửi 1 lượt sửa nhỏ (Medium + minor/nit).
- 2026-10-13 M0-T03 xong (3 commit, +757 không tính lock — vượt size S; 126k token, 74 tool call, ~14 phút). Cổng: check-workers 24 pass, sec rc=0, đúng scope. Lệch AC5: dev chạy watchfiles dạng hàm (lý do Windows). Đang review.
- 2026-10-13 Wave 1: giao T02 (be), T03 (be), T04 (fe) song song từ main 1838342. T07 không chạy sớm vì trùng scope api/Makefile, workers/Makefile. Mặc định: Makefile app in thông điệp ASCII tiếng Anh; chấp nhận pin Node 22.23.3, Python 3.12.15, uv 0.13.0.
- 2026-10-13 Owner yêu cầu orchestrator tự merge: M0-T01 fast-forward vào main local (99e11af), không push. T02, T03, T04 → ready.
- 2026-10-13 M0-T01 vòng 2: review-agent APPROVE (1 nit), security-agent PASS (2 Low, 2 Info). Status in-review, chờ Owner merge local (chưa có PR). Tổng token: be-agent 175k, review 62k+32k, security 39k+41k. Tổng wave 0 ≈ 390k token.
- 2026-10-13 M0-T01 review vòng 1: review-agent REQUEST_CHANGES (1 major: kiểm SR-2 trong make sec grep cả comment; 4 minor, 2 nit), security-agent PASS (3 Low, 1 Info). Gửi lại be-agent sửa vòng 1/2; chốt hợp đồng nạp .env cho Makefile con.
- 2026-10-13 M0-T01 xong (5 commit, +580, 148k token, 66 tool call, ~11 phút). Cổng: make help/check/sec rc=0, diff đúng scope. Đang review tự động.
- 2026-10-13 Commit đầu 733d9c0 (local, bỏ .claude/agent-memory/). Wave 0: giao M0-T01 cho be-agent, branch chore/M0-T01-repo-skeleton từ local main.
- 2026-10-13 Owner duyệt plan; câu 3–6 theo mặc định. Chế độ chạy: local, chưa PR (chưa có remote/gh).
- 2026-10-13 Owner chốt Go mới nhất (1.27.2) thay cho 1.23; cập nhật T01, T02.
- 2026-10-13 Lập plan. Threat model: 20 SR (7 High, 10 Medium, 3 Low), đã gắn vào task. Bỏ bước Explore vì repo chưa có code.
