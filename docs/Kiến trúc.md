# Kiến trúc hệ thống

Oct 9, 2026 · @Nhu Canh

Hệ thống gồm 3 khối chạy on-premise: Next.js (giao diện), Go API (xác thực, phân quyền, Settings, agent, truy xuất và model gateway), Python worker (crawl, parse, chunk, embed). Mọi model là model của LLM provider bên ngoài, chỉ Go được gọi ra qua egress proxy. Các khối nối qua Redis và dùng chung 3 kho: Postgres, Qdrant, MinIO; triển khai bằng Docker Compose trên 2 máy chủ, không cần GPU.

## 1. Sơ đồ tổng thể

&#91;embedded content: sơ đồ tổng thể · 4 khối xử lý, 3 kho, 1 cổng vào\]

Người dùng chỉ đi qua Nginx; Go API là nơi duy nhất kiểm quyền và nơi duy nhất gọi LLM provider. Worker Python cần embed thì gọi API nội bộ của model gateway trong Go, không giữ API key. File lớn đi thẳng từ trình duyệt vào MinIO bằng presigned URL do Go cấp.

| Thành phần | Trách nhiệm | Nói chuyện với |
| --- | --- | --- |
| Nginx | TLS, định tuyến, giới hạn kích thước request, giữ kết nối SSE | Next.js, Go API, MinIO |
| Next.js | Giao diện, gồm khu Settings; trình duyệt gọi `/api` trực tiếp | (không gọi backend từ server) |
| Go API | Xác thực, RBAC, Settings, KB, upload, agent, retrieval, chat SSE, Inspector, Eval, Admin | Redis, Postgres, Qdrant, MinIO |
| Model gateway (trong Go) | Adapter cho từng provider, retry, rate limit, đếm token và chi phí; API nội bộ `/internal/v1/embed`, `/internal/v1/chat` cho worker | LLM provider (qua egress proxy) |
| Go nền | Scheduler (health check provider, dọn KB đã xóa, đẩy lại job kẹt, tổng hợp chi phí) và eval-runner | Redis, Postgres, Qdrant, model gateway |
| Redis | Hàng đợi job (Streams), trạng thái realtime (Pub/Sub), cache quyền, rate limit | Go, worker |
| Python workers | Crawl URL, kiểm tra + parse file, chunk, tính BM25, embed + upsert, chấm Eval | Redis, Postgres, Qdrant, MinIO, ClamAV, model gateway của Go |
| Egress proxy | Cổng ra Internet duy nhất: chỉ cho domain của provider đã khai báo và crawl; ghi log | Go, worker-crawl |
| LLM provider (bên ngoài) | Chat, utility, embedding, rerank (OCR, vision ở P1) | Được model gateway gọi qua HTTPS |
| PostgreSQL | Nguồn sự thật: user, quyền, model, KB, tài liệu, chunk, hội thoại, trace, eval, chi phí, audit | — |
| Qdrant | Vector dense + sparse (BM25), payload để lọc theo kb\_id | — |
| MinIO | File gốc, kết quả parse (JSON có vị trí), file xuất | — |

## 2. Tech stack chốt

Phiên bản ghi là mức tối thiểu; bản cụ thể được pin trong `go.mod`, `pyproject.toml`, `package.json` và tag image khi khởi tạo repo.

| Tầng | Công nghệ | Phiên bản tối thiểu | Vai trò / lý do chọn |
| --- | --- | --- | --- |
| Frontend | Next.js (App Router) + TypeScript | Next.js 15, Node.js 22 LTS | UI chat, KB, Inspector, Eval, Settings, Admin |
| Frontend | Tailwind CSS + shadcn/ui | Tailwind 4 | Bộ component nhanh, tự sở hữu code |
| Frontend | TanStack Query, react-pdf (PDF.js), next-intl, react-hook-form + zod | — | Cache dữ liệu; tô sáng trích dẫn; vi/en; form các trang Settings |
| Backend | Go | 1.27 (dùng bản ổn định mới nhất) | REST, SSE, agent loop, model gateway; đồng thời cao |
| Backend | chi, pgx + sqlc, go-redis, qdrant go-client, minio-go | — | Router; SQL type-safe; client cho từng kho |
| Backend | golang-jwt, argon2id; coreos/go-oidc (P1) | — | JWT RS256, hash mật khẩu; Google OAuth ở Phase 2 |
| Model gateway | openai-go, anthropic-sdk-go, google.golang.org/genai; REST client cho Cohere, Voyage, Jina | — | SDK chính thức cho chat + tool calling; embed và rerank qua REST |
| Xử lý tài liệu | Python | 3.12 | Worker crawl, parse, chunk, embed, eval |
| Xử lý tài liệu | Docling (parser chính), PyMuPDF (PDF nhanh, kiểm trang có text) | — | Một thư viện cho PDF, DOCX, PPTX, XLSX, HTML, giữ bố cục và bảng |
| Xử lý tài liệu | Playwright + trafilatura | — | Render trang JS, lọc nội dung chính |
| Xử lý tài liệu | lingua, tiktoken, httpx, redis-py, psycopg 3, qdrant-client, minio | — | Phát hiện ngôn ngữ, đếm token, gọi gateway, client kho |
| Tìm kiếm từ khóa | Tokenizer BM25 tự viết theo đặc tả chung (Go + Python) + sparse vector Qdrant với IDF modifier | Qdrant 1.10+ | Provider không trả sparse vector; tokenizer giống hệt lúc index và lúc truy vấn |
| LLM provider | OpenAI, Azure OpenAI, Anthropic, Google Gemini, OpenAI-compatible; Cohere, Voyage AI, Jina AI cho embedding và rerank | — | Khai báo trong Settings; không host model, không GPU |
| Hàng đợi, cache | Redis | 7.2 | Streams (job), Pub/Sub (trạng thái), cache quyền, rate limit |
| CSDL | PostgreSQL | 16 | Nguồn sự thật |
| Vector DB | Qdrant | 1.11 | Dense + sparse trong cùng point; payload index tenant theo kb\_id |
| File | MinIO | bản 2024 trở đi | File gốc, kết quả parse; API S3 |
| Quét malware | ClamAV (clamd) | 1.3 | Quét file trước khi parse |
| Cổng vào | Nginx | 1.26 | TLS, định tuyến `/` và `/api`, cấu hình cho SSE |
| Cổng ra | Squid (egress proxy) | 6 | Chỉ cho phép domain provider và crawl, ghi log lưu lượng ra ngoài |
| Quan sát | OpenTelemetry Collector, Prometheus, Grafana, Loki, Tempo | — | Metrics, log, trace, dashboard chi phí |
| Đóng gói, triển khai | Docker, Docker Compose v2 | Docker 27 | Một file compose cho dev và production |
| CI | GitLab CI hoặc GitHub Actions + registry nội bộ (Harbor) | — | Build, test, quét, đẩy image |

Model đề xuất để đưa vào Eval (chưa chốt; kiểm tra bản mới hơn lúc triển khai): **embedding** OpenAI text-embedding-3-large, Cohere embed-multilingual-v3.0 hoặc Gemini embedding; **rerank** Cohere Rerank 3.5 hoặc Jina Reranker v2 multilingual; **chat** một model mạnh có tool calling của OpenAI, Anthropic hoặc Gemini; **utility** model nhỏ, rẻ cùng provider. Điều kiện: tốt tiếng Việt, Anh, Trung; chọn theo kết quả Eval trên bộ tài liệu mẫu và chi phí.

## 3. Cấu trúc code

Một monorepo, 3 ứng dụng (`web`, `server`, `workers`) dùng chung thư mục `contracts` để không lệch định dạng dữ liệu giữa Go, Python và TypeScript. Đặc tả tokenizer BM25 và bộ test vector cũng nằm trong `contracts` để Go và Python cho cùng kết quả.

**Monorepo**

```
agentic-rag/
├── web/                     # Next.js
├── server/                  # Go: API, agent, model gateway, scheduler, eval-runner
├── workers/                 # Python: crawl, parse, chunk, embed, eval
├── contracts/
│   ├── openapi.yaml         # API REST → sinh client TS và type Go
│   ├── internal-api.yaml    # API nội bộ model gateway cho worker
│   ├── jobs/*.schema.json   # Job Redis → sinh model Pydantic và struct Go
│   ├── events/*.schema.json # Sự kiện SSE
│   ├── errors.yaml          # danh mục mã lỗi dùng chung (vi/en)
│   └── bm25/                # đặc tả tokenizer + test_vectors.json
├── deployment/
│   ├── docker-compose.yml
│   ├── docker-compose.dev.yaml
│   ├── .env.example
│   ├── nginx/nginx.conf
│   ├── squid/squid.conf     # egress proxy: allowlist domain provider
│   └── observability/
├── testdata/                # 50 tài liệu mẫu khó, eval dataset mẫu
├── docs/adr/
└── Makefile                 # make dev | test | gen | migrate | up
```

**Go API** (`server/`): chia theo domain, mỗi domain có `handler.go → service.go → repo.go`; domain gọi nhau qua interface của service.

```
server/
├── cmd/app/main.go          # app api | scheduler | eval-runner | migrate
├── internal/
│   ├── platform/            # config, log, db, redis, qdrant, minio, otel, httpx
│   ├── auth/                # đăng ký, đăng nhập, JWT, refresh (google/ ở Phase 2)
│   ├── authz/               # RBAC: CanAccessKB, AllowedKBIDs, cache quyền
│   ├── account/             # hồ sơ cá nhân (/me), đổi mật khẩu, ngôn ngữ, giao diện
│   ├── llm/
│   │   ├── provider.go      # interface ChatModel, Embedder, Reranker
│   │   ├── openai/          # OpenAI, Azure OpenAI, OpenAI-compatible
│   │   ├── anthropic/
│   │   ├── gemini/
│   │   ├── cohere/  voyage/  jina/   # embedding, rerank
│   │   ├── registry/        # provider, model, mã hóa key, model mặc định
│   │   └── gateway/         # retry, rate limit, ưu tiên, đếm token, chi phí, API nội bộ
│   ├── textproc/bm25/       # tokenizer BM25 theo contracts/bm25
│   ├── workspace/           # workspace, KB, thành viên
│   ├── document/            # upload, trạng thái, xóa, retry
│   ├── queue/               # Redis Streams: Publisher, Consumer
│   ├── retrieval/           # hybrid search, RRF, rerank, filter quyền
│   ├── agent/               # vòng lặp ReAct, tool, prompt, kiểm trích dẫn
│   ├── chat/                # hội thoại, message, feedback, SSE
│   ├── trace/  inspector/  eval/  admin/
├── migrations/
├── queries/                 # SQL cho sqlc
├── gen/
└── Dockerfile
```

**Python worker** (`workers/`): mỗi stage là hàm `run(job, ctx) -> next_job`; phần nhận job, ack, retry, ghi sự kiện nằm trong `runtime/`. Worker không chứa API key: mọi lời gọi model đi qua `clients/gateway.py`.

```
workers/
├── rag_worker/
│   ├── __main__.py          # python -m rag_worker --stage parse
│   ├── runtime/             # consumer Redis Streams, retry, DLQ, heartbeat, OTel
│   ├── contracts/           # Pydantic sinh từ contracts/jobs
│   ├── stages/              # crawl.py  parse.py  chunk.py  embed.py
│   ├── parsers/             # docling_parser.py  pdf_fast.py  html.py
│   ├── chunkers/            # heading.py  fixed_token.py  slide_sheet.py
│   ├── textproc/bm25.py     # tokenizer BM25 theo contracts/bm25
│   ├── clients/             # postgres  minio  qdrant  clamd  gateway
│   └── eval/                # judge.py  generate.py (gọi chat qua gateway)
├── tests/
├── pyproject.toml
└── Dockerfile
```

**Next.js** (`web/`)

```
web/
├── app/
│   ├── (auth)/login, register, invite, forgot-password
│   ├── (app)/chat/[conversationId]
│   ├── (app)/kb/[kbId]/documents, documents/[docId] (Inspector), eval
│   ├── (app)/settings/
│   │   ├── profile/         # hồ sơ cá nhân, đổi mật khẩu, ngôn ngữ, giao diện
│   │   ├── members/         # thành viên workspace / KB; System Admin: mọi user
│   │   ├── models/          # cấu hình Model: provider, API key, model, mặc định
│   │   ├── kb/[kbId]/       # chung, chunking, model riêng, prompt agent
│   │   ├── quota/  audit/   # System Admin
│   │   └── security/        # P1: đăng ký, mật khẩu, Google OAuth
│   └── (app)/admin/usage            # dashboard chi phí
├── components/              # ui/ (shadcn), chat/, citation-viewer/, inspector/, settings-form/
├── lib/api/                 # client sinh từ openapi.yaml, xử lý refresh token
├── lib/sse/                 # EventSource + reconnect Last-Event-ID
├── messages/                # vi.json, en.json
└── Dockerfile
```

## 4. Vòng đời dữ liệu

&#91;embedded content: vòng đời dữ liệu · 10 bước\]

File gốc và Postgres là bản gốc; kết quả parse, chunk và vector đều là dữ liệu dẫn xuất có phiên bản (`version` khi đổi file, `generation` khi re-index). Mỗi bước 1–5 ghi một sự kiện vào `pipeline_events` để Inspector hiển thị.

**Dữ liệu rời công ty ở đâu:** bước 5 gửi nội dung chunk tới embedding provider; bước 6 gửi câu hỏi tới embedding và câu hỏi + chunk tới rerank provider; bước 7 gửi câu hỏi, lịch sử hội thoại và chunk tới chat provider; Eval gửi cả đáp án chuẩn. Hệ thống không lưu gì ở provider; thời gian provider giữ dữ liệu phụ thuộc hợp đồng (ưu tiên zero data retention). File gốc, parse và vector luôn nằm trong công ty.

**Dữ liệu nằm ở đâu, giữ bao lâu**

| Dữ liệu | Tạo ở bước | Lưu ở | Giữ bao lâu | Khi xóa tài liệu / KB |
| --- | --- | --- | --- | --- |
| Bản ghi tài liệu (tên, nguồn, hash, trạng thái) | 1 | Postgres `documents` | Theo tài liệu | Xóa mềm, xóa hẳn sau 30 ngày |
| File gốc | 2 | MinIO `rag-raw/kb/{kb}/doc/{doc}/v{n}/` | Theo tài liệu, giữ các version | Xóa mọi version |
| HTML snapshot (URL) | 2 | MinIO `rag-crawl/` | 90 ngày | Xóa |
| Kết quả parse (text, bảng, vị trí trang) | 3 | MinIO `rag-parsed/.../v{n}.json` | Version hiện tại + 1 version trước | Xóa |
| Chunk (text, token, trang, heading, ngôn ngữ) | 4 | Postgres `chunks` | Version hiện tại | Xóa theo `document_id` |
| Vector + payload | 5 | Qdrant collection theo embedding model | Generation hiện tại; generation cũ 7 ngày | Xóa theo filter `doc_id` / `kb_id` |
| Sự kiện pipeline | 1–5 | Postgres `pipeline_events` (partition tháng) | 180 ngày | Giữ đến hết hạn |
| Hội thoại, câu trả lời, trích dẫn | 7 | Postgres `conversations`, `messages`, `citations` | Theo chính sách (mặc định 1 năm) | Trích dẫn hiển thị “nguồn đã bị xóa” |
| Trace | 6–7 | Postgres `traces`, `trace_spans` (partition tháng) | 90 ngày | Giữ đến hết hạn |
| Feedback, Eval dataset, kết quả Eval | 8 | Postgres `feedback`, `eval_*` | Theo KB | Xóa cùng KB |
| Job, trạng thái realtime, cache quyền | 1–7 | Redis | Phút đến ngày (TTL) | Tự hết hạn |
| Audit log | Mọi hành động | Postgres `audit_logs` | 2 năm | Không xóa |

**Trạng thái của một tài liệu qua các bước 1–5**

&#91;embedded content: trạng thái tài liệu · 11 trạng thái\]

**Quy trình xóa** (bước 10)

1. Go đánh dấu `Deleting`; tài liệu biến mất khỏi kết quả tìm kiếm ngay (filter theo trạng thái trong Postgres + xóa point Qdrant theo `doc_id`).
2. Job dọn dẹp xóa chunk trong Postgres và file trong MinIO.
3. Với KB: xóa mềm 30 ngày (khôi phục được), sau đó scheduler xóa hẳn cả 3 kho và ghi audit.

## 5. Triển khai với Docker

Một file `docker-compose.yml` chia thành 3 profile (`app`, `data`, `obs`) chạy trên 2 máy chủ, không cần GPU. Model là LLM provider bên ngoài, chỉ được gọi qua container `squid` (egress proxy). Dev chạy mọi profile trên 1 máy.

&#91;embedded content: triển khai Docker · 2 máy chủ + LLM provider bên ngoài\]

Máy dữ liệu chỉ mở cổng cho IP máy ứng dụng. Người dùng chỉ thấy cổng 443 của Nginx. Squid có 2 cổng: `:3128` cho model gateway (chỉ domain provider), `:3129` cho worker-crawl (mọi URL công khai, chặn dải IP nội bộ).

### 5.1 docker-compose.yml

```yaml
name: agentic-rag

x-app-env: &app-env
  POSTGRES_DSN: postgres://rag:${POSTGRES_PASSWORD}@${DATA_HOST}:5432/rag
  REDIS_ADDR: ${DATA_HOST}:6379
  REDIS_PASSWORD: ${REDIS_PASSWORD}
  QDRANT_ADDR: ${DATA_HOST}:6334
  QDRANT_API_KEY: ${QDRANT_API_KEY}
  MINIO_ENDPOINT: ${DATA_HOST}:9000
  MINIO_PUBLIC_ENDPOINT: https://s3.${PUBLIC_DOMAIN}
  MINIO_ACCESS_KEY: ${MINIO_ACCESS_KEY}
  MINIO_SECRET_KEY: ${MINIO_SECRET_KEY}
  OTEL_EXPORTER_OTLP_ENDPOINT: http://otel-collector:4317

x-go: &go
  image: ${REGISTRY}/rag-api:${APP_VERSION}
  profiles: [app]
  restart: unless-stopped
  environment:
    <<: *app-env
    HTTPS_PROXY: http://squid:3128          # mọi lời gọi LLM provider đi qua squid
    NO_PROXY: localhost,api,${DATA_HOST}
    MASTER_KEY_FILE: /run/secrets/master_key  # giải mã API key provider lưu trong Postgres
    INTERNAL_TOKEN_FILE: /run/secrets/internal_token
  secrets: [master_key, internal_token]
  networks: [backend]

x-worker: &worker
  image: ${REGISTRY}/rag-worker:${APP_VERSION}
  profiles: [app]
  restart: unless-stopped
  environment:
    <<: *app-env
    GATEWAY_URL: http://api:8080/internal/v1  # embed / chat qua Go, worker không giữ API key
    INTERNAL_TOKEN_FILE: /run/secrets/internal_token
    CLAMD_ADDR: clamd:3310
  secrets: [internal_token]
  networks: [backend]

services:
  # ---------- profile app (server ứng dụng) ----------
  nginx:
    image: nginx:1.26-alpine
    profiles: [app]
    ports: ["443:443", "80:80"]
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./nginx/certs:/etc/nginx/certs:ro
    depends_on: [web, api]
    networks: [backend]

  web:
    image: ${REGISTRY}/rag-web:${APP_VERSION}
    profiles: [app]
    restart: unless-stopped
    networks: [backend]

  api:
    <<: *go
    command: ["api"]
    environment:
      <<: *app-env
      HTTPS_PROXY: http://squid:3128
      NO_PROXY: localhost,api,${DATA_HOST}
      PUBLIC_URL: https://${PUBLIC_DOMAIN}
      JWT_PRIVATE_KEY_FILE: /run/secrets/jwt_private_key
      MASTER_KEY_FILE: /run/secrets/master_key
      INTERNAL_TOKEN_FILE: /run/secrets/internal_token
    secrets: [jwt_private_key, master_key, internal_token]
    deploy:
      replicas: 3
    healthcheck:
      test: ["CMD", "/app", "healthcheck"]
      interval: 10s
      retries: 3

  scheduler:
    <<: *go
    command: ["scheduler"]

  eval-runner:
    <<: *go
    command: ["eval-runner"]

  migrate:
    <<: *go
    profiles: [tools]
    restart: "no"
    command: ["migrate", "up"]

  worker-crawl:
    <<: *worker
    command: ["--stage", "crawl"]
    environment:
      <<: *app-env
      HTTPS_PROXY: http://squid:3129         # cổng crawl: mọi URL công khai, chặn IP nội bộ
      HTTP_PROXY: http://squid:3129
      NO_PROXY: ${DATA_HOST},api
      GATEWAY_URL: http://api:8080/internal/v1
      INTERNAL_TOKEN_FILE: /run/secrets/internal_token
  worker-parse:
    <<: *worker
    command: ["--stage", "parse"]
    deploy:
      replicas: 2
      resources:
        limits: { cpus: "2", memory: 4g }
  worker-chunk:
    <<: *worker
    command: ["--stage", "chunk"]
  worker-embed:
    <<: *worker
    command: ["--stage", "embed"]
  worker-eval:
    <<: *worker
    command: ["--stage", "eval"]

  clamd:
    image: clamav/clamav:stable
    profiles: [app]
    restart: unless-stopped
    volumes: [clamav-db:/var/lib/clamav]
    networks: [backend]

  squid:
    image: ubuntu/squid:${SQUID_TAG}
    profiles: [app]
    restart: unless-stopped
    volumes:
      - ./squid/squid.conf:/etc/squid/squid.conf:ro
      - squid-logs:/var/log/squid
    networks: [backend]

  # ---------- profile data (server dữ liệu) ----------
  postgres:
    image: postgres:16-alpine
    profiles: [data]
    restart: unless-stopped
    environment:
      POSTGRES_DB: rag
      POSTGRES_USER: rag
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    command: ["postgres", "-c", "max_connections=300", "-c", "shared_buffers=8GB"]
    ports: ["${DATA_BIND_IP}:5432:5432"]
    volumes: [pg-data:/var/lib/postgresql/data]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U rag"]
      interval: 10s

  redis:
    image: redis:7.2-alpine
    profiles: [data]
    restart: unless-stopped
    command: ["redis-server", "--appendonly", "yes", "--requirepass", "${REDIS_PASSWORD}", "--maxmemory-policy", "noeviction"]
    ports: ["${DATA_BIND_IP}:6379:6379"]
    volumes: [redis-data:/data]

  qdrant:
    image: qdrant/qdrant:v1.11.0
    profiles: [data]
    restart: unless-stopped
    environment:
      QDRANT__SERVICE__API_KEY: ${QDRANT_API_KEY}
    ports: ["${DATA_BIND_IP}:6333:6333", "${DATA_BIND_IP}:6334:6334"]
    volumes: [qdrant-data:/qdrant/storage]

  minio:
    image: minio/minio:${MINIO_TAG}
    profiles: [data]
    restart: unless-stopped
    command: ["server", "/data", "--console-address", ":9001"]
    environment:
      MINIO_ROOT_USER: ${MINIO_ROOT_USER}
      MINIO_ROOT_PASSWORD: ${MINIO_ROOT_PASSWORD}
    ports: ["${DATA_BIND_IP}:9000:9000", "${DATA_BIND_IP}:9001:9001"]
    volumes: [minio-data:/data]

  # ---------- profile obs (server ứng dụng) ----------
  otel-collector:
    image: otel/opentelemetry-collector-contrib:0.110.0
    profiles: [obs]
    volumes: [./observability/otel-collector.yaml:/etc/otelcol-contrib/config.yaml:ro]
    networks: [backend]
  prometheus:
    image: prom/prometheus:v2.54.1
    profiles: [obs]
    volumes: [./observability/prometheus.yml:/etc/prometheus/prometheus.yml:ro, prom-data:/prometheus]
    networks: [backend]
  grafana:
    image: grafana/grafana:11.2.0
    profiles: [obs]
    volumes: [grafana-data:/var/lib/grafana]
    networks: [backend]
  # loki, tempo khai báo tương tự

networks:
  backend: {}

volumes:
  pg-data: {}
  redis-data: {}
  qdrant-data: {}
  minio-data: {}
  clamav-db: {}
  squid-logs: {}
  prom-data: {}
  grafana-data: {}

secrets:
  jwt_private_key: { file: ./secrets/jwt_private_key.pem }
  master_key: { file: ./secrets/master_key }
  internal_token: { file: ./secrets/internal_token }
```

Phiên bản image trong file là mốc tham khảo; team pin bản ổn định mới nhất khi khởi tạo. **API key của provider không nằm trong `.env`**: System Admin nhập trên màn Settings → LLM provider, Go mã hóa bằng `master_key` rồi lưu Postgres. `internal_token` là token dùng chung giữa worker và model gateway; API `/internal/*` chỉ nhận request từ mạng `backend`, Nginx không chuyển `/internal` ra ngoài.

### 5.2 .env (trích)

```bash
REGISTRY=harbor.internal/agentic-rag
APP_VERSION=1.0.0
PUBLIC_DOMAIN=rag.company.internal
DATA_HOST=10.0.1.20        # IP LAN server dữ liệu
DATA_BIND_IP=10.0.1.20     # chỉ lắng nghe trên LAN, không 0.0.0.0
POSTGRES_PASSWORD=...
REDIS_PASSWORD=...
QDRANT_API_KEY=...
MINIO_TAG=RELEASE.<pin khi cài đặt>
MINIO_ROOT_USER=...
MINIO_ROOT_PASSWORD=...
MINIO_ACCESS_KEY=...       # user riêng cho ứng dụng, không dùng root
MINIO_SECRET_KEY=...
SQUID_TAG=<pin khi cài đặt>
# Không có API key của LLM provider ở đây: nhập trong Settings trên giao diện
```

File `.env` và thư mục `secrets/` không commit; quyền file 600, chỉ user vận hành đọc được.

### 5.3 Dockerfile

**Go API** (`server/Dockerfile`): build tĩnh, image distroless không có shell.

```dockerfile
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/app

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
COPY migrations /migrations
USER nonroot
ENTRYPOINT ["/app"]
```

**Python worker** (`workers/Dockerfile`)

```dockerfile
FROM python:3.12-slim
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1
RUN apt-get update && apt-get install -y --no-install-recommends libmagic1 \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY . .
RUN pip install --no-cache-dir . && playwright install --with-deps chromium
RUN useradd -u 10001 worker
USER worker
ENTRYPOINT ["python", "-m", "rag_worker"]
```

**Next.js** (`web/Dockerfile`, dùng `output: "standalone"`)

```dockerfile
FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:22-alpine
WORKDIR /app
ENV NODE_ENV=production
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static
COPY --from=build /app/public ./public
USER node
EXPOSE 3000
CMD ["node", "server.js"]
```

**Egress proxy** (`deployment/squid/squid.conf`): cổng 3128 chỉ cho domain của provider đang dùng, cổng 3129 cho crawl. Danh sách domain provider nên được sinh từ Settings mỗi khi thêm provider.

```
http_port 3128
http_port 3129
acl port_llm   myportname 3128
acl port_crawl myportname 3129
acl llm_providers dstdomain .openai.com .openai.azure.com .anthropic.com generativelanguage.googleapis.com .cohere.com .voyageai.com .jina.ai
acl private_net dst 10.0.0.0/8 172.16.0.0/12 192.168.0.0/16 127.0.0.0/8 169.254.0.0/16
http_access allow port_llm llm_providers
http_access deny  port_crawl private_net
http_access allow port_crawl
http_access deny  all
```

### 5.4 Nginx (phần quan trọng)

```nginx
resolver 127.0.0.11 valid=10s;          # DNS của Docker, chia tải giữa 3 container api
client_max_body_size 2m;                # file lớn đi thẳng MinIO, không qua api

server {
  listen 443 ssl;
  server_name rag.company.internal;

  location /api/ {
    set $api http://api:8080;
    proxy_pass $api;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header X-Request-ID $request_id;
    proxy_buffering off;                 # bắt buộc cho SSE
    proxy_read_timeout 1h;
  }

  location / {
    proxy_pass http://web:3000;
  }
}

server {
  listen 443 ssl;
  server_name s3.rag.company.internal;   # presigned URL của MinIO
  client_max_body_size 200m;
  location / { proxy_pass http://10.0.1.20:9000; proxy_request_buffering off; }
}
```

### 5.5 Vận hành

| Việc | Lệnh |
| --- | --- |
| Build và đẩy image (CI) | `make build push APP_VERSION=1.0.0` |
| Khởi động server dữ liệu | `docker compose --profile data up -d` |
| Chạy migration (mỗi lần nâng cấp) | `docker compose --profile tools run --rm migrate` |
| Khởi động server ứng dụng | `docker compose --profile app --profile obs up -d` |
| Cấu hình lần đầu | Đăng nhập tài khoản System Admin đầu tiên → Settings → LLM provider: thêm provider, API key, chọn model mặc định |
| Tăng worker khi hàng đợi dài | `docker compose up -d --scale worker-parse=4 --no-recreate` |
| Nâng cấp phiên bản | Đổi `APP_VERSION` → `docker compose pull` → migrate → `docker compose up -d` |
| Xem log | `docker compose logs -f api worker-parse squid` |
| Dev trên 1 máy | `make dev` (= `docker compose -f docker-compose.yml -f docker-compose.dev.yaml --profile app --profile data --profile obs up`, dùng mock provider) |

**Sao lưu** (cron trên server dữ liệu, đẩy sang máy backup riêng): `pg_dump` hằng ngày + WAL archive (WAL-G) cho Postgres; API snapshot của Qdrant hằng ngày; `mc mirror` cho MinIO. Redis không cần backup riêng: job mất sẽ được scheduler đẩy lại từ trạng thái trong Postgres.

**Giới hạn cần biết**: Docker Compose không tự failover khi cả máy chết và không rolling update hoàn toàn không gián đoạn. Hệ thống cũng phụ thuộc Internet: mất kết nối tới provider thì chat và embed dừng (job nằm chờ trong hàng đợi, không mất). Với tool nội bộ 500 user, đây là đánh đổi hợp lý; khi cần HA thật sự thì chuyển các image này sang Kubernetes mà không phải sửa code.
