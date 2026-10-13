# Agentic RAG

Monorepo gồm ba ứng dụng chạy trên máy dev, cùng Postgres và Redis chạy bằng Docker:

| Thư mục | Vai trò | Công nghệ |
| --- | --- | --- |
| `api/` | API nghiệp vụ | Go 1.27 |
| `workers/` | Worker xử lý nền | Python 3.12 + uv |
| `web/` | Giao diện | Next.js 15 + TypeScript |
| `deploy/` | Compose dev, mẫu biến môi trường | Docker Compose |
| `docs/` | PRD, kiến trúc, SPEC, plan, ADR | Markdown |

Repo được dựng dần theo từng module (xem [Nguyên tắc cấu trúc tăng dần](#nguyên-tắc-cấu-trúc-tăng-dần)).
Ứng dụng chưa có thư mục hoặc `Makefile` thì các lệnh `make` tương ứng in `skip: <app> chưa có` và bỏ qua.

## Từ máy trắng tới chạy được

### 1. Cài công cụ

| Công cụ | Dùng để | Ghi chú |
| --- | --- | --- |
| Docker Desktop | Chạy Postgres, Redis | Mở Docker Desktop và chờ engine chạy trước khi `make up` |
| Git | Clone repo | Windows: cài Git for Windows để có **Git Bash** |
| make | Chạy các lệnh của repo | GNU make 3.81 trở lên. Cách cài ở mục 2 (Windows) và 3 (macOS/Linux) |
| mise (khuyến nghị) | Cài đúng phiên bản Go, Node, Python, uv và các công cụ quét | Không dùng mise thì cài tay theo bảng "Cài tay" bên dưới |

**Cài tay (không dùng mise).** Cài đúng các phiên bản trong `mise.toml` (file này là nguồn duy nhất, bảng dưới có thể cũ hơn):

| Công cụ | Phiên bản |
| --- | --- |
| Go | 1.27.2 |
| Node.js | 22.23.3 |
| Python | 3.12.15 |
| uv | 0.13.0 |
| gitleaks | 8.30.1 |
| golangci-lint | 2.14.0 |
| govulncheck (`go install golang.org/x/vuln/cmd/govulncheck@v1.9.0`) | 1.9.0 |
| air (`go install github.com/air-verse/air@v1.67.4`) | 1.67.4 |

Cài từ trang phát hành chính thức hoặc trình quản lý gói của hệ điều hành. Không chạy script tải từ mạng
rồi chuyển thẳng vào shell.

### 2. Windows (Git Bash + Docker Desktop)

Mọi lệnh trong README chạy trong **Git Bash** (hoặc WSL2 như Linux), không chạy trong PowerShell hay cmd.

1. Cài Docker Desktop và Git for Windows.
2. Cài `make` bằng một trong ba cách (mở terminal mới sau khi cài):

   ```bash
   winget install ezwinports.make     # winget
   choco install make                 # Chocolatey
   scoop install make                 # Scoop
   ```

   Kiểm tra trong Git Bash: `make --version`.
3. Cài mise: `winget install jdx.mise` (hoặc `scoop install mise`), rồi thêm vào `~/.bashrc` của Git Bash:

   ```bash
   eval "$(mise activate bash)"
   ```

4. Nếu Git cảnh báo `LF will be replaced by CRLF`: repo đã có `.gitattributes` ép LF, bạn không cần đổi gì.

### 3. macOS / Linux

```bash
# macOS (make có sẵn qua Xcode Command Line Tools; brew chỉ cần cho mise)
brew install mise

# Linux: cài make và mise bằng trình quản lý gói của bản phân phối,
# ví dụ: sudo apt install make   (mise: xem https://mise.jdx.dev/installing-mise.html)
```

Thêm `eval "$(mise activate bash)"` (hoặc `zsh`) vào file khởi động shell.

### 4. Chạy

```bash
git clone <url-repo> agentic-rag && cd agentic-rag
mise install          # cài toolchain đúng phiên bản (bỏ qua nếu đã cài tay)
make env              # tạo .env từ deploy/.env.example (không ghi đè nếu đã có)
make up               # Postgres 16 + Redis 7.2, chờ healthy mới trả về
```

Sau đó mở **3 terminal** (đều ở gốc repo) khi các ứng dụng đã có:

```bash
make dev-api          # terminal 1: Go API, http://127.0.0.1:8080
make dev-worker       # terminal 2: Python worker
make dev-web          # terminal 3: Next.js, http://localhost:3000
```

Kiểm tra: `make ps` thấy Postgres và Redis `healthy`, cổng dạng `127.0.0.1:5432->5432/tcp`.
Trước khi mở PR: `make check` và `make sec`.

### 5. Gỡ rối

| Triệu chứng | Cách xử lý |
| --- | --- |
| `Cannot connect to the Docker daemon` / `error during connect` | Docker Desktop chưa chạy. Mở nó, chờ engine sẵn sàng |
| `port is already allocated` | Cổng 5432 hoặc 6379 đang bị dùng. Đổi `POSTGRES_PORT` hoặc `REDIS_PORT` trong `.env` (và cổng trong `DATABASE_URL`, `REDIS_URL`) |
| `required variable ... is missing` khi chạy `docker compose` trực tiếp | Compose cần biến từ `.env`. Dùng `make ps`, `make up`, `make down`, hoặc thêm `--env-file .env` |
| `make: command not found` (Windows) | Cài `make` (mục 2) rồi mở lại Git Bash |
| `make sec` báo thiếu gitleaks | `mise install`, hoặc cài tay đúng phiên bản trong `mise.toml` |
| Đổi mật khẩu trong `.env` mà DB vẫn dùng mật khẩu cũ | Volume giữ mật khẩu lần khởi tạo đầu. Chạy `make reset` (xóa dữ liệu dev) |

## Bảng lệnh

Chạy `make` hoặc `make help` để xem danh sách luôn đúng với Makefile.

| Lệnh | Việc làm |
| --- | --- |
| `make help` | Liệt kê các lệnh (mặc định) |
| `make env` | Tạo `.env` từ `deploy/.env.example` nếu chưa có (không bao giờ ghi đè) |
| `make up` | Chạy Postgres 16 + Redis 7.2 (chỉ bind `127.0.0.1`), chờ healthy |
| `make down` | Dừng Postgres + Redis, giữ dữ liệu |
| `make ps` | Xem trạng thái Postgres + Redis |
| `make reset` | Hỏi `y/N`; nếu `y` thì xóa volume dữ liệu của project `agentic-rag-dev` rồi `up` lại. Không đụng gì ngoài project này |
| `make dev-api` / `dev-worker` / `dev-web` | Chạy ứng dụng ở chế độ dev (ủy quyền cho `<app>/Makefile`, target `dev`) |
| `make check` | Chạy `check-api`, `check-workers`, `check-web` |
| `make check-api` / `check-workers` / `check-web` | Lint, typecheck, test của từng ứng dụng |
| `make sec` | Quét bảo mật, dừng ngay khi có lỗi (xem bên dưới) |

`make sec` làm lần lượt, bước nào lỗi hoặc có phát hiện thì trả mã khác 0 (không bỏ qua lỗi):

1. Kiểm công cụ: thiếu `gitleaks` hoặc `docker` thì báo lỗi kèm cách cài.
2. Kiểm compose: mọi cổng `published` đều bind `127.0.0.1`, không `POSTGRES_HOST_AUTH_METHOD`, Redis có `--requirepass`, mọi image pin tag phiên bản (redis từ 7.2.11).
3. `gitleaks` quét lịch sử git và các file git nhìn thấy trong cây làm việc (kể cả file chưa track, trừ file bị `.gitignore`), luôn với `--redact`.
4. `make -C <app> sec` cho từng app đã có Makefile (govulncheck, pip-audit, npm audit... do app định nghĩa).

## Cấu hình và an toàn khi dev

- Cấu hình nằm ở `.env` gốc repo (`make env` tạo từ `deploy/.env.example`). `.env` bị `.gitignore` chặn; **không commit** `.env`, `deploy/secrets/`, `*.pem`.
- Mọi giá trị trong `deploy/.env.example` là giả và **chỉ dùng cho dev**. Secret thật (key LLM provider...) không bao giờ đặt ở đây hay trong biến `NEXT_PUBLIC_*`.
- Postgres và Redis chỉ nghe ở `127.0.0.1` và bắt buộc mật khẩu (kể cả trên máy dev). Kiểm tra: `docker exec agentic-rag-dev-redis-1 redis-cli PING` phải trả `NOAUTH`.
- Image Docker và toolchain pin phiên bản cụ thể. Nâng phiên bản là một thay đổi riêng, kèm `make check` và `make sec`.

## Thêm ứng dụng/area

Makefile gốc **không biết** chi tiết từng app, chỉ gọi `make -C <app> <target>` nếu `<app>/Makefile` tồn tại.
Mỗi app (`api/`, `workers/`, `web/`) tự thêm `Makefile` của mình, **không sửa Makefile gốc**. Hợp đồng:

| Target | Bắt buộc | Yêu cầu |
| --- | --- | --- |
| `check` | Có | Lint + typecheck + test của app. Trả mã khác 0 nếu có lỗi. Chạy độc lập, không cần dịch vụ Docker |
| `dev` | Có | Chạy app với hot reload, **bind `127.0.0.1`** (vd. `next dev -H 127.0.0.1`). Đọc cấu hình từ `.env` gốc |
| `sec` | Có | Quét phụ thuộc theo lockfile (`govulncheck ./...`, `uv run pip-audit`, `npm audit --audit-level=high`). Fail-closed: không nuốt mã lỗi của lệnh quét (không tiền tố `-`, không "hoặc true"), thiếu công cụ thì báo lỗi |
| `test-integration` | Tùy chọn | Test cần Docker (testcontainers...). Chạy riêng bằng `make -C <app> test-integration`, không nằm trong `check` |

Ghi chú:

- Dùng chế độ khóa lockfile khi cài phụ thuộc: `npm ci`, `uv sync --frozen`, `go mod download` + `go mod verify`.
- Makefile con nên chỉ chứa ký tự ASCII trong lệnh (recipe). `make` bản Windows làm hỏng ký tự UTF-8 khi truyền lệnh cho bash; thông điệp tiếng Việt hãy đặt ở file riêng hoặc dùng cách của Makefile gốc (dòng `#msg:` ở cuối file).
- Khi thêm lệnh mới ở gốc, ghi mô tả sau `##` trên dòng khai báo target để `make help` tự hiển thị.
- Service Docker mới (MinIO, Qdrant...) do module cần tới thêm vào `deploy/docker-compose.dev.yml`, ghim tag phiên bản và bind `127.0.0.1`.

## Nguyên tắc cấu trúc tăng dần

M0 chỉ tạo những gì mọi module sau đều cần ngay: thư mục ba ứng dụng, bộ lệnh `make`, lint/test, Postgres + Redis local.
Thư mục, thư viện, service Docker và file contract còn lại do **module đầu tiên cần tới** thêm vào, trong cùng PR với tính năng đó,
đúng vị trí và tên trong `docs/Kiến trúc.md` mục 3. Không thêm `.gitkeep` hay thư mục rỗng "để dành".

Chi tiết và lý do: [ADR 0001](docs/adr/0001-incremental-structure.md). Đặc tả: `docs/specs/SPEC-M0-platform.md`.

## Tài liệu

- `docs/PRD v1.md`, `docs/Kiến trúc.md`: yêu cầu sản phẩm và kiến trúc đích.
- `docs/specs/`, `docs/plans/`: SPEC, plan và task theo module.
- `docs/adr/`: các quyết định kiến trúc.
