# Hướng dẫn setup dự án và workflow coding agent

Oct 10, 2026 · @Nhu Canh

Tài liệu này hướng dẫn hai việc:

1. Dựng monorepo Agentic RAG theo [Kiến trúc](Kiến%20trúc.md) để chạy được trên máy dev.
2. Cài đặt quy trình code bằng nhiều agent Claude Code.

Quy trình gồm các bước:

- **SPEC** được chia thành **task nhỏ FE/BE**.
- Các **agent chuyên biệt** (`fe-agent`, `be-agent`, `test-agent`, `security-agent`) làm việc song song, do một **`orchestrator`** điều phối.
- Từng PR qua **review tự động** (`review-agent` + `security-agent`).
- **Owner review và merge từng PR nhỏ.**

Toàn bộ file cấu hình agent nằm sẵn trong thư mục `agent-kit/`, chép vào gốc repo là dùng được.

---

## 1. Tổng quan quy trình

```
 Owner viết SPEC (docs/specs/SPEC-xxx.md)
        │
        ▼
 /plan-spec ──► security-agent (threat model) ∥ Explore (bản đồ code)      [chỉ đọc, song song]
        │
        ▼
 docs/plans/SPEC-xxx/plan.md + tasks/*.md        ⏸ ĐIỂM DỪNG 1: Owner duyệt plan
        │
        ▼
 /run-wave SPEC-xxx 0   (contracts + migration nền)  ──► PR ──► Owner merge
        │
        ▼
 /run-wave SPEC-xxx 1
   ┌───────────┬───────────┬───────────┬────────────┐
 fe-agent    be-agent    be-agent    test-agent           mỗi agent 1 git worktree + 1 branch
   └───────────┴─────┬─────┴───────────┴────────────┘
                     ▼
     Orchestrator chạy cổng chất lượng: make check-<area>   (fail → trả agent sửa 1 lần)
                     ▼
     push + PR draft (1 task = 1 PR)
                     ▼
     review-agent ∥ security-agent ──(blocker/major, Critical/High)──► agent sửa (≤ 2 vòng)
                     │ sạch
                     ▼
     PR "ready-for-human"                      ⏸ ĐIỂM DỪNG 2: Owner review từng PR
        │ comment                         │ approve
        ▼                                 ▼
 /address-comments <PR>            Owner merge ──► /run-wave SPEC-xxx (wave tiếp)
```

| Bước             | Ai làm                                  | Đầu vào                       | Đầu ra                                  | Owner can thiệp     |
| ---------------- | --------------------------------------- | ----------------------------- | --------------------------------------- | ------------------- |
| Viết SPEC        | Owner (có thể nhờ Claude nháp)          | PRD, Phân rã P0, UI canvas    | `docs/specs/SPEC-*.md`                  | Viết và chốt        |
| Lập plan         | orchestrator + security-agent + Explore | SPEC                          | plan, task, threat model                | **Duyệt plan**      |
| Wave 0           | be-agent                                | task contracts                | PR thay đổi `contracts/`, migration nền | Review kỹ, merge    |
| Wave 1..n        | fe/be/test-agent song song              | task + code sinh từ contracts | Mỗi task một PR                         | —                   |
| Review tự động   | review-agent ∥ security-agent           | diff của PR                   | Comment trên PR, verdict                | —                   |
| Review của người | Owner                                   | PR `ready-for-human`          | Approve, comment, merge                 | **Quyết định cuối** |

---

## 2. Chuẩn bị máy

Trên Windows nên làm việc trong **WSL2 (Ubuntu 24.04)**, với Docker Desktop dùng WSL2 backend. Lý do: `make`, bash, hook và bind mount Docker chạy nhanh và ổn định hơn nhiều so với chạy thẳng trên Windows.

| Công cụ                              | Phiên bản                               | Ghi chú                                                                              |
| ------------------------------------ | --------------------------------------- | ------------------------------------------------------------------------------------ |
| Git                                  | ≥ 2.40                                  | Cần cho `git worktree`                                                               |
| Docker Engine / Desktop + Compose v2 | Docker 27                               |                                                                                      |
| Go                                   | 1.26                                    | Kèm `golangci-lint`, `sqlc`, `govulncheck`, `oapi-codegen` (cài bằng `make tools`)   |
| Node.js                              | 22 LTS                                  | npm (theo Dockerfile của web)                                                        |
| Python                               | 3.12 + [uv](https://docs.astral.sh/uv/) |                                                                                      |
| GNU make, openssl, jq                | —                                       |                                                                                      |
| GitHub CLI `gh`                      | ≥ 2.50                                  | `gh auth login`; orchestrator dùng nó để mở PR và comment                            |
| Bảo mật                              | gitleaks, semgrep, trivy                | Dùng trong `make sec` và CI                                                          |
| **Claude Code**                      | bản mới nhất                            | Cài: `curl -fsSL https://claude.ai/install.sh \| bash`, sau đó `claude` để đăng nhập |

---

## 3. Khởi tạo repo

### 3.1 Tạo monorepo và chép agent-kit

```bash
mkdir agentic-rag && cd agentic-rag
git init -b main
mkdir -p web api workers contracts/{jobs,events,bm25} deploy/{nginx,squid,observability,secrets} testdata docs/{specs,plans,adr} tools
cp -r "/mnt/f/Agentic Rag/agent-kit/." .          # .claude/, .github/, CLAUDE.md, docs/specs/_TEMPLATE.md
cp "/mnt/f/Agentic Rag/docs/"*.md docs/            # Kiến trúc, PRD, Phân rã P0, Kiến thức AI, tài liệu này
printf '.env\n.env.*\n!.env.example\ndeploy/secrets/\n*.pem\n.claude/settings.local.json\n.claude/agent-memory-local/\nnode_modules/\n.next/\n__pycache__/\n.venv/\n' > .gitignore
```

Sau khi chép, cấu trúc phần agent như sau:

```
.claude/
├── settings.json              # quyền allow/ask/deny, hook toàn cục, giới hạn subagent
├── hooks/guard-bash.mjs       # chặn lệnh nguy hiểm (push main, merge, đọc secret…)
├── agents/                    # 6 agent
│   ├── orchestrator.md  fe-agent.md  be-agent.md
│   └── test-agent.md    security-agent.md  review-agent.md
└── skills/
    ├── task-contract/  contracts-first/                   # dùng chung
    ├── fe-ui-design/  fe-nextjs-typescript/               # fe-agent
    ├── be-go-standards/  be-python-standards/             # be-agent
    ├── testing-standards/  security-standards/  code-review-standards/
    └── plan-spec/  run-wave/  review-pr/  address-comments/  task-status/   # lệnh /… của Owner
.github/  pull_request_template.md  CODEOWNERS  workflows/ci.yml
CLAUDE.md
docs/specs/_TEMPLATE.md
```

### 3.2 Khung ba ứng dụng (task M0-01, có thể giao luôn cho agent)

```bash
# web
npx create-next-app@15 web --ts --app --tailwind --eslint --src-dir=false --import-alias "@/*" --use-npm
cd web && npx shadcn@latest init && npx shadcn@latest add button input card badge tabs table dialog dropdown-menu switch progress sidebar breadcrumb tooltip skeleton alert alert-dialog select form sonner && cd ..
# api
cd api && go mod init github.com/<org>/agentic-rag/api && cd ..
# workers
cd workers && uv init --package --name rag-worker --python 3.12 && cd ..
```

### 3.3 Makefile gốc

Các lệnh dưới đây là "giao diện" chung cho người, agent và CI. Agent chỉ cần biết `make check-<area>`.

```make
COMPOSE = docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.dev.yml --env-file deploy/.env
.PHONY: tools dev gen migrate check check-web check-api check-workers sec e2e

tools:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

dev:            ## toàn bộ stack + mock LLM provider
	$(COMPOSE) --profile data --profile app --profile obs up --build
gen:            ## sinh code từ contracts/ cho TS, Go, Python
	./tools/gen.sh
migrate:
	$(COMPOSE) --profile tools run --rm migrate

check-web:
	cd web && npm run lint && npm run typecheck && npm test -- --run && npm run build
check-api:
	cd api && test -z "$$(gofmt -l .)" && golangci-lint run && sqlc diff \
	  && go test -race ./... && go test -race -tags integration ./...
check-workers:
	cd workers && uv run ruff check && uv run ruff format --check \
	  && uv run mypy --strict rag_worker && uv run pytest -q
check: check-web check-api check-workers

sec:
	gitleaks detect --no-banner
	semgrep ci --config p/owasp-top-ten --config p/golang --config p/python --config p/typescript --config p/secrets
	cd api && govulncheck ./...
	cd workers && uv run pip-audit
	cd web && npm audit --audit-level=high
	trivy fs --severity HIGH,CRITICAL --exit-code 1 .

e2e:
	$(COMPOSE) --profile data --profile app up -d --wait && cd web && npx playwright test
```

### 3.4 Secret cho môi trường dev

```bash
cp deploy/.env.example deploy/.env           # điền mật khẩu dev; DATA_HOST=localhost
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:3072 -out deploy/secrets/jwt_private_key.pem
openssl rand -base64 32 > deploy/secrets/master_key
openssl rand -hex 32    > deploy/secrets/internal_token
chmod 600 deploy/.env deploy/secrets/*
```

Như đã chốt trong Kiến trúc mục 5, API key của LLM provider **không** nằm trong `.env`. Môi trường dev dùng **mock provider** (OpenAI-compatible) trong `docker-compose.dev.yml`, nên không tốn tiền API. Agent và test cũng không bao giờ gọi provider thật.

### 3.5 Chạy thử

```bash
make tools && make gen && make dev           # cửa sổ khác:
make migrate && make check
```

Mở `https://localhost`, đăng nhập bằng tài khoản System Admin đầu tiên, rồi vào Settings → Cấu hình Model và thêm mock provider.

### 3.6 GitHub: bảo vệ nhánh main

1. Tạo repo trên GitHub (hoặc GitHub Enterprise nội bộ), rồi `git remote add origin … && git push -u origin main`.
2. Vào **Settings → Rules → Rulesets**, tạo ruleset cho `main`:
   - bắt buộc qua PR;
   - tối thiểu **1 approval**, **bắt buộc review của Code Owners**;
   - tự hủy approval cũ khi có commit mới;
   - bắt buộc resolve hết hội thoại;
   - status check bắt buộc: **`gate`** (job tổng của `ci.yml`), branch phải cập nhật với main;
   - chặn force push và xóa nhánh;
   - chỉ cho **squash merge**, tự xóa branch sau khi merge.
3. Sửa `.github/CODEOWNERS`, thay `<owner>` bằng username thật. Thư mục `/.claude/` chỉ Owner duyệt, vì sửa ở đó là đổi hành vi của mọi agent.
4. Tạo các nhãn: `agent`, `ready-for-human`, `needs-human`, `blocked-by-deps`, `e2e`, `area/web`, `area/api`, `area/workers`, `area/contracts`, `wave/0`…`wave/5`.
   ```bash
   for l in agent ready-for-human needs-human blocked-by-deps e2e area/web area/api area/workers area/contracts wave/0 wave/1 wave/2 wave/3; do gh label create "$l" --force; done
   ```

Dùng GitLab thay GitHub: giữ nguyên quy trình, đổi `gh pr …` thành `glab mr …` trong các skill `run-wave`, `review-pr`, `address-comments`, `task-status`, và chuyển `ci.yml` sang `.gitlab-ci.yml`.

---

## 4. Đội agent

| Agent              | Vai trò                                                                    | Model / effort | Công cụ                | Skill chuyên biệt                                                            | Cách ly                     | Được làm / không được làm                                                               |
| ------------------ | -------------------------------------------------------------------------- | -------------- | ---------------------- | ---------------------------------------------------------------------------- | --------------------------- | --------------------------------------------------------------------------------------- |
| **orchestrator**   | Tech lead điều phối: lập plan, giao việc, cổng chất lượng, mở PR, báo cáo  | opus / high    | Tất cả                 | task-contract, contracts-first + 5 lệnh workflow                             | Chạy làm phiên chính        | Chỉ sửa `docs/plans/`; được push branch task và mở PR; **không merge, không push main** |
| **fe-agent**       | Dựng UI đẹp, chuẩn Next.js 15 + TS strict                                  | sonnet / high  | Tất cả                 | **fe-ui-design**, **fe-nextjs-typescript**, task-contract, contracts-first   | worktree riêng              | Chỉ sửa `web/`; không push                                                              |
| **be-agent**       | Go (API, gateway, agent, migration) + Python (worker), sở hữu `contracts/` | sonnet / high  | Tất cả                 | **be-go-standards**, **be-python-standards**, contracts-first, task-contract | worktree riêng              | Sửa `api/`, `workers/`, `contracts/`; không push                                        |
| **test-agent**     | Test tích hợp, contract, e2e, test RAG từ AC                               | sonnet / high  | Tất cả                 | **testing-standards**, task-contract, contracts-first                        | worktree riêng              | Chỉ sửa thư mục test và `testdata/`; không sửa code sản phẩm                            |
| **security-agent** | Threat model lúc lập plan và review bảo mật mỗi PR                         | opus / high    | Read, Grep, Glob, Bash | **security-standards**                                                       | Chỉ đọc (hook `--readonly`) | Không sửa, không commit; có memory dự án                                                |
| **review-agent**   | Review chất lượng code mỗi PR                                              | opus / high    | Read, Grep, Glob, Bash | **code-review-standards** + chuẩn FE/BE                                      | Chỉ đọc                     | Không sửa; có memory dự án                                                              |

**Vì sao chia như vậy:**

- Agent viết code dùng sonnet để nhanh và rẻ.
- Agent đánh giá (review, security) dùng opus và nhận chỉ dẫn khác hẳn, để không "chấm bài của chính mình" với cùng góc nhìn.
- Agent đánh giá chỉ đọc, nên không thể tự sửa cho qua.

**Mở rộng sau này:**

- Thêm `devops-agent` (skill Docker/Nginx/Squid/CI) cho task `area: deploy`.
- Tách `worker-agent` khỏi `be-agent` khi khối Python lớn lên.

Cách thêm agent: tạo file trong `.claude/agents/` và skill tương ứng, rồi thêm vào bảng "Đội agent" trong `orchestrator.md`.

---

## 5. Bộ skill chuẩn enterprise

Skill là file hướng dẫn chuyên sâu, được **nạp sẵn** vào agent qua trường `skills:`. Agent mang theo đúng phần kiến thức nó cần, không phải nhồi cả vào một prompt khổng lồ.

| Skill                                                                   | Dùng bởi               | Nội dung chính                                                                                                                                                                                                                                       |
| ----------------------------------------------------------------------- | ---------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `fe-ui-design`                                                          | fe-agent               | Token màu (#435CFF, ink, border…), typography Geist, lưới 4px, bo góc, shadcn + Lucide, mẫu Sidebar/avatar/Settings popup, layout 1440×900 cuộn bên trong, 4 trạng thái dữ liệu, motion 150–200ms, WCAG 2.2 AA, checklist tự kiểm                    |
| `fe-nextjs-typescript`                                                  | fe-agent, review-agent | Cấu trúc `app/`, `components/`, `features/`; TS strict; Server và Client component (trình duyệt gọi `/api` trực tiếp); client sinh từ OpenAPI; TanStack Query keys; form rhf+zod; SSE; render markdown LLM an toàn; hiệu năng; Vitest/MSW/Playwright |
| `be-go-standards`                                                       | be-agent, review-agent | Domain handler→service→repo; chi, pgx, sqlc; RFC 9457 problem+json; RBAC chống IDOR; migration expand/contract; context, timeout, errgroup; LLM chỉ qua gateway; slog, OTel, metric; test dạng bảng, testcontainers, `-race`                         |
| `be-python-standards`                                                   | be-agent, review-agent | Stage `run(job, ctx)` idempotent; Retryable và Permanent error, DLQ; Pydantic sinh từ contracts; xử lý file không tin cậy; gateway client; mypy strict, ruff; tokenizer BM25 khớp Go                                                                 |
| `contracts-first`                                                       | mọi agent              | Bảng nguồn → code sinh ra; `make gen`; quy ước REST; tương thích ngược và khi nào cần ADR                                                                                                                                                            |
| `task-contract`                                                         | agent viết code        | File task là hợp đồng; phạm vi; git, Conventional Commits; Definition of Done; **định dạng báo cáo** trả về orchestrator                                                                                                                             |
| `testing-standards`                                                     | test-agent             | Kim tự tháp; truy vết AC → test; tính xác định; **test RAG bắt buộc** (rò quyền giữa KB, trích dẫn, prompt injection, golden set, BM25, idempotency); e2e tối thiểu; xử lý flaky                                                                     |
| `security-standards`                                                    | security-agent         | ASVS L2, OWASP Top 10, API Top 10, **LLM Top 10**; 14 mối đe dọa riêng của dự án (IDOR, prompt injection gián tiếp, SSRF crawl, upload, egress dữ liệu ra provider…); `make sec`; mức độ và định dạng                                                |
| `code-review-standards`                                                 | review-agent           | Checklist 11 mục; mức blocker/major/minor/nit; định dạng kết quả; phát hiện phải dựa trên code đã đọc                                                                                                                                                |
| `plan-spec`, `run-wave`, `review-pr`, `address-comments`, `task-status` | Owner gọi bằng `/…`    | Các bước của workflow (mục 6)                                                                                                                                                                                                                        |

Cập nhật skill như cập nhật code: qua PR, Owner duyệt. Khi review-agent hoặc Owner thấy một lỗi lặp lại hai lần, thêm luật vào skill tương ứng để agent không lặp lại lần thứ ba.

---

## 6. Workflow chi tiết

### 6.0 Mở phiên Orchestrator

```bash
cd agentic-rag
claude --agent orchestrator
```

Phiên chính chạy với system prompt của `orchestrator`. Lệnh `initialPrompt` khiến nó tự đọc `docs/plans/` và báo trạng thái ngay khi mở. Từ đây Owner chỉ cần gõ các lệnh `/…`.

### 6.1 Viết SPEC

Chép `docs/specs/_TEMPLATE.md` thành `docs/specs/SPEC-M1-auth.md` và điền.

- **Mỗi SPEC nên vừa 1–2 tuần công việc**, tương đương một module hoặc một phần module trong Phân rã P0.
- AC viết dạng **Given / When / Then** để agent và test chuyển thẳng thành test.
- Ghi rõ màn hình trong UI canvas, quyền theo vai trò, thay đổi API và dữ liệu, và **dữ liệu nào đi ra LLM provider**.

Có thể nhờ Claude nháp SPEC từ PRD, nhưng Owner chốt.

### 6.2 `/plan-spec docs/specs/SPEC-M1-auth.md`

Orchestrator:

1. **Kiểm SPEC đủ điều kiện (Definition of Ready).** Thiếu AC, mâu thuẫn, chưa rõ quyền thì trả câu hỏi và dừng.
2. Chạy song song `security-agent` (threat model STRIDE, sinh `SR-n`) và `Explore` (bản đồ code bị ảnh hưởng).
3. Chia task theo các luật:
   - **1 task = 1 PR = 1 owner = 1 area**, size S (≤150 dòng) hoặc M (≤400 dòng).
   - **Wave 0 = contracts** (và migration nền), merge trước.
   - Từ wave 1, FE và BE chạy song song dựa trên code sinh ra từ contracts.
   - Không có hai task cùng wave sửa chung file.
   - Task test của test-agent chạy song song với task code.
4. Ghi `plan.md`, `tasks/*.md`, `threat-model.md` và trình bảng wave.

**⏸ Owner duyệt plan.** Xem các điểm sau:

- cách chia có hợp lý không;
- AC của từng task có đúng ý không;
- SR có đủ không.

Sửa trực tiếp file task nếu cần, rồi ghi ngày vào ô "Duyệt bởi Owner" trong `plan.md`.

### 6.3 `/run-wave SPEC-M1-auth 0`, rồi các wave tiếp theo

Orchestrator:

1. Đồng bộ trạng thái PR với GitHub, dọn worktree của task đã merge.
2. Chọn tối đa 4 task chạy được: phụ thuộc đã merge, phạm vi không giao nhau.
3. **Giao tất cả trong một lượt** để chúng chạy song song.
   - Mỗi agent có worktree riêng (`isolation: worktree`), tạo branch `feat/<TASK-ID>-<slug>` từ `origin/main`, code, test, commit.
   - Agent **không push** (hook chặn).
4. Với từng task xong:
   - **tự chạy lại `make check-<area>`**, không tin báo cáo của agent;
   - kiểm diff có nằm trong phạm vi không;
   - rồi `git push` và mở **PR draft** theo template.
5. Chạy `review-agent` ∥ `security-agent` cho mọi PR của wave, rồi đăng kết quả thành comment.
   - Có lỗi chặn thì gửi lại cho agent viết code sửa, **tối đa 2 vòng**, sau đó gắn nhãn `needs-human`.
6. PR sạch thì chuyển `ready`, gắn nhãn `ready-for-human`, và báo Owner bằng bảng.

**Khi nào được chạy song song:**

| Tình huống                                      | Song song? | Vì sao                                                           |
| ----------------------------------------------- | ---------- | ---------------------------------------------------------------- |
| UI đăng nhập (fe) và API đăng nhập (be)         | ✅         | FE dùng client sinh từ contracts + MSW, không chờ BE             |
| API đăng nhập (be) và test tích hợp auth (test) | ✅         | Test viết từ AC và contract; PR test giữ draft đến khi API merge |
| Hai task cùng sửa `api/internal/auth/`          | ❌         | Xung đột file; xếp sang 2 wave                                   |
| Đổi `openapi.yaml` và code dùng nó              | ❌         | Contract phải merge trước (wave 0)                               |

**Những file hay xung đột và cách tránh:**

- **i18n:** tách theo tính năng, `messages/vi/<feature>.json`.
- **Migration:** đặt tên theo timestamp (`20261010_1200_create_sessions.up.sql`), không đánh số tăng dần.
- **Đăng ký route:** mỗi domain có `routes.go` riêng.
- **Lockfile:** chỉ task được phép thêm dependency mới sửa lockfile.

### 6.4 Review tự động

|               | review-agent                                   | security-agent                                                           |
| ------------- | ---------------------------------------------- | ------------------------------------------------------------------------ |
| Đối chiếu với | Task contract, AC, convention FE/BE, kiến trúc | Threat model, SR, 14 mối đe dọa riêng, OWASP                             |
| Công cụ       | Đọc code, chạy test để kiểm chứng              | `make sec` (gitleaks, semgrep, govulncheck, pip-audit, npm audit, trivy) |
| Mức chặn      | blocker, major                                 | Critical, High                                                           |
| Kết luận      | `APPROVE` / `REQUEST_CHANGES`                  | `PASS` / `FAIL`                                                          |

Mọi phát hiện phải có `file:line` và cách sửa. Nghi ngờ chưa kiểm chứng được thì xếp vào mục "Owner nên xem kỹ", không xếp thành lỗi chặn.

### 6.5 Owner review từng PR

PR đến tay Owner đã: CI xanh, review tự động sạch hoặc còn mục `minor`, có bảng AC → bằng chứng. Owner nên dành thời gian cho những gì máy làm kém:

1. **Đúng ý nghiệp vụ không?** Đọc mục "Thay đổi" và "AC → bằng chứng", chạy thử nếu là UI.
2. **Mục "Owner nên xem kỹ"** mà review-agent nêu.
3. **Thiết kế và đánh đổi:** có tạo nợ kỹ thuật, có đặt tiền lệ xấu không.
4. **Test có kiểm đúng hành vi không**: đọc tên test và assert, không chỉ nhìn coverage.
5. PR đụng `contracts/`, `migrations/`, `authz/`, `/.claude/`: đọc kỹ từng dòng.

Sau khi xem:

- **Approve và merge** (squash).
- **Request changes** bằng comment trên dòng code, rồi trong phiên orchestrator gõ `/address-comments <số PR>`.
  - Orchestrator giao agent sửa đúng các comment đó, mỗi comment một commit.
  - Sau đó chạy cổng chất lượng và review tự động lại, trả lời từng comment, rồi re-request review.
  - Comment mang tính thảo luận sẽ được trả lại để Owner quyết, agent không tự sửa.

### 6.6 Sang wave tiếp theo

Merge xong thì gõ `/run-wave SPEC-M1-auth` (không cần số wave). Orchestrator tự nhận các task đã đủ điều kiện. `/task-status` cho bảng toàn cảnh bất cứ lúc nào.

### 6.7 Ví dụ: SPEC-M1-auth

| Wave | Task                                                                                                             | Agent | Size | Phụ thuộc                      |
| ---- | ---------------------------------------------------------------------------------------------------------------- | ----- | ---- | ------------------------------ |
| 0    | M1-T01 Contract: `/auth/register, login, refresh, logout, verify-email, forgot/reset-password` + mã lỗi `AUTH_*` | be    | S    | —                              |
| 0    | M1-T02 Migration `users`, `sessions`, `email_tokens` + argon2id helper                                           | be    | S    | —                              |
| 1    | M1-T03 API đăng nhập (JWT RS256 + refresh cookie)                                                                | be    | M    | T01, T02                       |
| 1    | M1-T05 API đăng ký + gửi email xác thực                                                                          | be    | M    | T01, T02                       |
| 1    | M1-T09 UI đăng nhập/đăng ký theo `Login.dc.html`                                                                 | fe    | M    | T01                            |
| 1    | M1-T10 Test tích hợp + contract test auth                                                                        | test  | M    | T01 (ready khi T03, T05 merge) |
| 2    | M1-T04 Refresh rotation + phát hiện dùng lại                                                                     | be    | M    | T03                            |
| 2    | M1-T06 Quên/đổi mật khẩu                                                                                         | be    | S    | T05                            |
| 2    | M1-T11 UI tự refresh token + quên mật khẩu                                                                       | fe    | M    | T03, T06                       |
| 3    | M1-T07 Rate limit + khóa tạm                                                                                     | be    | S    | T04                            |
| 3    | M1-T08 Middleware RBAC + test quét route thiếu quyền                                                             | be    | M    | T03                            |
| 3    | M1-T12 E2E auth + test rò quyền                                                                                  | test  | M    | T08, T11                       |

Kết quả: 12 PR nhỏ thay vì 1 PR khổng lồ. Mỗi wave có 2–4 agent chạy cùng lúc.

---

## 7. Guardrails: nhiều lớp, không chỉ dựa vào prompt

| Lớp                              | Cơ chế                                                                                                                                                                                                         | Chặn gì                                                                                                        |
| -------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| 1. Prompt agent                  | Phạm vi và luật trong `agents/*.md`, `task-contract`                                                                                                                                                           | Hướng hành vi, nhưng **không đảm bảo**                                                                         |
| 2. Quyền công cụ                 | review-agent và security-agent chỉ có Read, Grep, Glob, Bash                                                                                                                                                   | Không sửa file bằng Edit/Write                                                                                 |
| 3. Permissions (`settings.json`) | `deny`: `gh pr merge`, force push, đọc `.env`, `deploy/secrets/`, `*.pem`. `ask`: `git push`, `gh pr create/review/edit`, `gh api`                                                                             | Owner xác nhận mỗi lần push hoặc mở PR. Khi đã tin quy trình, chuyển `git push` và `gh pr create` sang `allow` |
| 4. Hook `guard-bash.mjs`         | Toàn cục: merge, force push, push main, `--no-verify`, đọc secret, `rm -rf /`, `docker compose down -v`. `--implementer`: chặn push và mọi lệnh ghi lên GitHub. `--readonly`: chặn thêm git commit và ghi file | **Chặn chắc chắn** bằng code, không phụ thuộc agent có "nghe lời" hay không                                    |
| 5. Worktree                      | Mỗi agent một thư mục làm việc                                                                                                                                                                                 | Agent không ghi đè việc của nhau                                                                               |
| 6. Cổng chất lượng               | Orchestrator tự chạy `make check-*`                                                                                                                                                                            | Agent báo xong mà thật ra chưa xong                                                                            |
| 7. CI + branch protection        | Job `gate`, CODEOWNERS, 1 approval                                                                                                                                                                             | Không có gì vào `main` mà không qua Owner                                                                      |

Thêm vài giới hạn:

- `CLAUDE_CODE_MAX_SUBAGENT_SPAWN_DEPTH=1`: agent con không được tự sinh thêm agent, tránh bùng nổ chi phí.
- `CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS=8`: tối đa 4 agent code cộng với các reviewer.

**Lưu ý trên Windows:** hook chạy bằng `node`, nên cần Node trong PATH. Nếu chạy Claude Code ngoài WSL, hook vẫn bắt cả lệnh từ công cụ PowerShell (matcher `Bash|PowerShell`).

---

## 8. Vận hành và cải tiến

**Chỉ số theo dõi hằng tuần**, ghi trong nhật ký `plan.md` hoặc dashboard:

| Chỉ số                                  | Mục tiêu               | Nếu xấu thì                            |
| --------------------------------------- | ---------------------- | -------------------------------------- |
| Thời gian từ PR mở đến merge            | < 1 ngày               | Task quá to: chia nhỏ hơn              |
| Tỉ lệ PR qua review tự động ngay vòng 1 | > 60%                  | Bổ sung skill FE/BE từ các lỗi lặp lại |
| Tỉ lệ PR Owner request changes          | < 30%                  | AC mơ hồ: viết SPEC kỹ hơn             |
| Số PR gắn `needs-human`                 | Giảm dần               | Xem lỗi gì mà agent không tự sửa được  |
| Chi phí token mỗi task                  | Theo dõi bằng `/usage` | Hạ effort hoặc đổi model cho task S    |

**Vòng cải tiến:**

- Mỗi lần Owner comment một lỗi mà agent lẽ ra phải biết, thêm một dòng vào skill tương ứng.
- `review-agent` và `security-agent` có `memory: project`. Chúng tự ghi lại convention đã được xác nhận và rủi ro đã được Owner chấp nhận, trong `.claude/agent-memory/`; thư mục này được commit và review như code.

**Agent teams:** Claude Code có tính năng _agent teams_ (thử nghiệm, bật bằng `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1`), trong đó các agent nhắn tin trực tiếp cho nhau và dùng chung một task list. Quy trình này **chưa cần** tính năng đó: task đã được thiết kế độc lập qua contracts, và subagent cùng worktree ổn định hơn. Có thể cân nhắc lại khi cần các agent tranh luận với nhau, ví dụ debug một lỗi khó theo nhiều giả thuyết.

**Review trên GitHub (tùy chọn):** có thể thêm GitHub Action `anthropics/claude-code-action` để gọi `@claude` ngay trong PR. Không bắt buộc, vì orchestrator đã review trên máy.

---

## 9. Kiến thức AI liên quan

**1. Mẫu Orchestrator–Worker (điều phối và thợ).**

- Một agent "trưởng" nhận việc lớn, chia nhỏ, giao cho các agent "thợ" chuyên một mảng, rồi gom kết quả.
- Ưu điểm:
  - mỗi thợ chỉ cần ngữ cảnh hẹp nên làm tốt hơn;
  - chạy song song nên nhanh hơn.
- Đánh đổi:
  - tốn token hơn nhiều lần so với một agent;
  - chất lượng phụ thuộc hoàn toàn vào **lời giao việc**. Vì vậy `run-wave` có mẫu giao việc cố định, và `task-contract` có mẫu báo cáo cố định.

**2. Context window và cách ly ngữ cảnh.**

- Mỗi subagent bắt đầu với ngữ cảnh **trống**: không thấy hội thoại của orchestrator, chỉ thấy lời giao việc, system prompt của nó, CLAUDE.md và skill được nạp sẵn.
- Đó là ưu điểm, vì ngữ cảnh sạch giúp agent tập trung, tránh tình trạng ngữ cảnh dài làm chất lượng giảm ("context rot").
- Hệ quả: mọi thông tin agent cần phải nằm trong file task hoặc lời giao việc. Đây là lý do file task phải "tự đủ nghĩa".

**3. Skill và nạp kiến thức theo tầng (progressive disclosure).**

- Bình thường Claude chỉ thấy _mô tả_ ngắn của skill và tự nạp phần thân khi cần.
- Với agent chuyên biệt, ta **nạp sẵn** (`skills:` trong frontmatter) để chắc chắn agent luôn có chuẩn FE/BE trong đầu.
- Tách nhiều skill nhỏ thay vì một prompt khổng lồ giúp:
  - mỗi agent chỉ mang phần cần thiết;
  - sửa chuẩn ở một chỗ mà mọi agent dùng nó đều được cập nhật.

**4. Contracts-first: hợp đồng mở đường cho song song.**

- Hai agent chỉ làm song song an toàn khi đã thống nhất "giao diện" giữa chúng. Ở đây giao diện là OpenAPI, JSON Schema của job và event.
- Giống team người: chốt API trước thì FE và BE không phải chờ nhau.
- Code sinh tự động từ một nguồn giúp ba ngôn ngữ không bao giờ lệch kiểu dữ liệu.

**5. Git worktree.**

- Một repo có thể có nhiều thư mục làm việc, mỗi thư mục ở một branch, dùng chung lịch sử git.
- Mỗi agent làm trong worktree riêng nên không giẫm lên file của nhau. Ví dụ không có chuyện agent A chạy `npm install` trong lúc agent B đang build.

**6. Hướng dẫn bằng prompt và ràng buộc bằng code.**

- Prompt chỉ là _xác suất_: LLM thường làm theo, nhưng không phải lúc nào cũng làm theo.
- Những gì không được phép xảy ra (merge, push main, đọc secret) phải bị chặn bằng **cơ chế xác định**: permission, hook, branch protection.
- Nguyên tắc: _prompt để hướng dẫn, code để đảm bảo_.

**7. LLM làm reviewer: mạnh ở đâu, yếu ở đâu.**

- **Mạnh:** đọc nhanh toàn bộ diff, bắt lỗi theo checklist, đối chiếu AC, không mệt.
- **Yếu:**
  - có thể "bịa" lỗi;
  - cùng một model dễ có cùng điểm mù với người viết;
  - không hiểu ý đồ nghiệp vụ ngầm.
- Cách giảm:
  - reviewer dùng model và chỉ dẫn khác với người viết;
  - bắt buộc trích `file:line`;
  - chỉ lỗi đã kiểm chứng mới được chặn;
  - **con người quyết định cuối**.

**8. Con người trong vòng lặp (human-in-the-loop) và PR nhỏ.**

- Hai điểm dừng (duyệt plan, duyệt PR) đặt đúng chỗ con người có giá trị nhất: hiểu nghiệp vụ và đánh đổi.
- Nghiên cứu về review code (SmartBear/Cisco) cho thấy hiệu quả tìm lỗi giảm mạnh khi một lần review quá khoảng 400 dòng. Vì vậy task bị giới hạn ở size S/M.

**9. Chi phí và chọn model.**

- Chi phí của agent tỉ lệ với số token đọc vào (file, diff, skill) và sinh ra.
- Model mạnh (opus) dùng cho việc cần suy luận sâu và ít lần: lập plan, review, bảo mật. Model nhanh (sonnet) dùng cho việc viết code lặp lại nhiều.
- Mức `effort` điều chỉnh độ "suy nghĩ" của model. Task S đơn giản có thể hạ xuống `medium` để tiết kiệm.

---

## 10. Hạn chế cần biết

- **Agent vẫn có thể sai một cách tự tin.** Cổng chất lượng, CI và review của Owner là bắt buộc, không phải hình thức.
- **Song song chỉ hiệu quả khi task thật sự độc lập.** SPEC có nhiều phụ thuộc chéo sẽ chạy thành nhiều wave tuần tự, chậm hơn mong đợi. Khi đó nên cải thiện cách chia, không phải tăng số agent.
- **Permission prompt từ agent chạy nền hiện ở phiên chính.** Owner cần ngồi gần lúc chạy wave, hoặc chuyển các lệnh an toàn sang `allow`.
- **Chi phí tăng theo số agent và số vòng review.** Theo dõi `/usage`; giới hạn 4 agent code mỗi wave và 2 vòng sửa là để kiểm soát chi phí.
- **Worktree cần repo có remote `origin` và nhánh mặc định `main`.** Worktree không có thay đổi sẽ tự xóa; worktree đã có commit được giữ lại đến khi orchestrator dọn sau khi merge.
- **Định nghĩa agent và skill là code.** Sửa sai một skill có thể làm cả đội agent sai theo, nên mọi thay đổi trong `/.claude/` phải qua PR do Owner duyệt.
