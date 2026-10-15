# Phân rã P0 thành module

Oct 9, 2026 · @Nhu Canh

P0 được chia thành 11 module, 84 task, ước lượng sơ bộ khoảng 52 người-tuần. Với team 5 người (2 Go, 1 Python, 1 Frontend, 1 DevOps/QA), Phase 1 mất khoảng 12–14 tuần tính cả dự phòng. So với bản trước: thêm M10 Settings, M3 đổi thành kết nối LLM provider + model gateway, đăng nhập Google chuyển sang Phase 2.

## Tổng quan module

| Module | Phạm vi | Yêu cầu PRD | Stack chính | Phụ thuộc | Ước lượng (người-tuần) |
| --- | --- | --- | --- | --- | --- |
| M0 Platform & DevOps | Repo, CI/CD, Docker Compose, hạ tầng dữ liệu, egress proxy, observability nền | 7.1, 7.2, 7.4 | DevOps, Go | — | 4 |
| M1 Auth & RBAC | Đăng ký, đăng nhập email + mật khẩu, JWT/refresh, middleware quyền | AUTH-01–03, 05–06 | Go, Next.js | M0 | 3 |
| M10 Settings | Khu giao diện Settings: Hồ sơ, Thành viên, Cấu hình Model, KB, Quota, Audit | 4.5 | Go, Next.js | M1 | 3 |
| M2 Workspace & KB | Workspace, KB, thành viên, cấu hình chunk/model, xóa mềm, re-index | KB-01–06 | Go, Next.js | M1, M3 | 3 |
| M3 LLM Provider & gateway | Provider, model, adapter chat/embed/rerank, gateway (retry, rate limit, chi phí), API nội bộ cho worker | MDL-01–06 | Go, Next.js | M0 | 4 |
| M4 Ingestion pipeline | Upload, hàng đợi, parse, crawl, chunk, tokenizer BM25, embed, trạng thái realtime | ING-01–11 | Python, Go | M2, M3 | 9 |
| M5 Retrieval & Agent | Hybrid search, rerank, tool, agent loop, trích dẫn, bảo vệ injection | AGT-01–09 | Go | M3, M4 | 7 |
| M6 Chat | UI chat, streaming, lịch sử, trích dẫn, feedback, gợi ý, xuất | CHT-01–11 | Next.js, Go | M5 | 6 |
| M7 Inspector & Trace | Document Inspector, truy vấn thử, Chat Trace | INS-01–06, TRC-01–04 | Next.js, Go | M4, M5 | 5 |
| M8 Eval | Dataset, sinh câu hỏi, chạy eval, chỉ số, so sánh | EVL-01–06 | Go, Python, Next.js | M5 | 5 |
| M9 Admin, quota & audit | Quản lý user, áp quota và ngân sách, rate limit, audit log | ADM-01–04 | Go, Next.js | M1 | 3 |

Ước lượng là sơ bộ để lập kế hoạch; team cần tự ước lại khi chia sprint.

## Thứ tự triển khai

&#91;embedded content: thứ tự triển khai · 4 milestone, 11 module\]

M4 và M5 chiếm 16/52 người-tuần và nằm nối tiếp nhau, nên cần người làm chính ngay từ MS1 (dựng khung worker, thử parser, thử function calling của LLM đã chọn). Frontend có thể làm UI M6, M7 trước với API giả lập. Số tuần là sơ bộ cho team 5 người.

## M0. Platform & DevOps

Nền móng cho mọi module: môi trường chạy được từ ngày đầu, CI chặn code lỗi, log và trace có sẵn.

| Mã | Task | Stack | Phụ thuộc |
| --- | --- | --- | --- |
| M0-01 | Cấu trúc monorepo: `web/` (Next.js), `server/` (Go), `contracts/`, `workers/` (Python), `deployment/`, `docs/`; quy ước code, lint | Tất cả | — |
| M0-02 | Docker Compose dev: Postgres, Redis, Qdrant, MinIO, mock LLM provider (OpenAI-compatible) để dev không tốn tiền API | DevOps | M0-01 |
| M0-03 | Khung Go API: router, config, logging JSON, error format chuẩn, health/readiness, graceful shutdown | Go | M0-01 |
| M0-04 | Khung Python worker: consumer Redis Streams (consumer group, ack, retry, DLQ), config, logging | Python | M0-02 |
| M0-05 | Migration Postgres có phiên bản (golang-migrate); schema khởi tạo theo PRD mục 8.5 | Go | M0-03 |
| M0-06 | OpenTelemetry + Prometheus metrics cho Go và Python; trace\_id đi xuyên qua job Redis | Go, Python | M0-03, M0-04 |
| M0-07 | CI: build, test, lint, quét SAST/dependency, build image; CD lên môi trường staging | DevOps | M0-01 |
| M0-08 | docker-compose production theo profile (app, data, gpu, obs), Nginx TLS, Docker secrets, Dockerfile cho 4 image; tài liệu cài đặt | DevOps | M0-07 |

**Xong khi:** `docker compose up` chạy đủ stack; mỗi PR chạy CI xanh; staging tự deploy từ nhánh main.

## M1. Auth & RBAC

Xác thực và kiểm quyền đặt ở Go; mọi module sau dùng chung middleware này.

| Mã | Task | Stack | Phụ thuộc |
| --- | --- | --- | --- |
| M1-01 | Bảng `users`, `sessions`; hash mật khẩu argon2id; chính sách mật khẩu đọc từ cấu hình triển khai | Go | M0-05 |
| M1-02 | API đăng ký + gửi email xác thực (SMTP nội bộ); chế độ đăng ký tự do theo domain hoặc chỉ qua lời mời | Go | M1-01 |
| M1-03 | API đăng nhập: JWT access (ký RS256), refresh trong cookie httpOnly; thời hạn đọc từ cấu hình triển khai | Go | M1-01 |
| M1-04 | Refresh rotation + phát hiện dùng lại (thu hồi cả family); đăng xuất | Go | M1-03 |
| M1-06 | Quên / đổi mật khẩu qua token email 1 lần, hạn 30 phút | Go | M1-02 |
| M1-07 | Rate limit đăng nhập (Redis) + khóa tạm theo ngưỡng cấu hình | Go | M1-03 |
| M1-08 | Middleware RBAC: đọc `memberships`, kế thừa Workspace → KB, cache quyền Redis TTL 60 giây; hàm `CanAccessKB`, `AllowedKBIDs` | Go | M1-03 |
| M1-09 | UI: đăng ký, đăng nhập, chấp nhận lời mời, quên mật khẩu, tự refresh token | Next.js | M1-03 |

M1-05 (đăng nhập Google OAuth) đã chuyển sang Phase 2 theo AUTH-04.

**Xong khi:** đủ test cho token rotation và reuse; user bị thu hồi quyền mất truy cập trong tối đa 60 giây; mọi endpoint mới bắt buộc khai báo quyền (test tự động quét route thiếu).

## M2. Workspace & Knowledge Base

Quản lý vòng đời KB và cấu hình mà pipeline và agent sẽ đọc.

| Mã | Task | Stack | Phụ thuộc |
| --- | --- | --- | --- |
| M2-01 | API CRUD Workspace + KB (tên, mô tả, ngôn ngữ chính, tag) | Go | M1-08 |
| M2-02 | API thành viên: mời theo email, gán/đổi role, gán theo group; chặn Admin nâng quyền quá Contributor; luôn đúng 1 Owner | Go | M2-01 |
| M2-03 | Cấu hình chunking theo KB (chiến lược, size, overlap) lưu trong knowledge\_bases.chunk\_config, mặc định lấy từ cấu hình hệ thống | Go | M2-01 |
| M2-04 | Gán model embedding/chat/rerank cho KB; khóa embedding sau lần index đầu | Go | M2-01, M3-07 |
| M2-05 | Job re-index: đẩy lại toàn bộ tài liệu qua pipeline với generation mới, đổi active\_generation khi xong, dọn point cũ sau 7 ngày | Go, Python | M2-04, M4-09 |
| M2-06 | Xóa mềm KB 30 ngày + job dọn dẹp (MinIO, Qdrant, Postgres) | Go | M2-01 |
| M2-07 | UI: danh sách Workspace/KB, tạo KB, trang cài đặt KB (chung, thành viên, chunking, model) | Next.js | M2-01–04 |

**Xong khi:** test quyền cho từng ô của ma trận phân quyền (PRD mục 3) đều pass; re-index 1 KB mẫu không làm gián đoạn chat.

## M3. LLM Provider & model gateway

Không có model local: mọi lời gọi model đi qua một gateway trong Go, kể cả từ worker Python. Gateway lo chuẩn hóa API của các provider, retry, rate limit, đếm token và chi phí; API key chỉ nằm ở Go.

| Mã | Task | Stack | Phụ thuộc |
| --- | --- | --- | --- |
| M3-01 | Bảng `model_providers`, `models`, `model_usage`; mã hóa API key AES-256-GCM với khóa chính từ secret bên ngoài | Go | M0-05 |
| M3-02 | Adapter chat (stream + tool calling) cho OpenAI / Azure OpenAI / OpenAI-compatible, Anthropic, Gemini bằng SDK chính thức; chuẩn hóa về 1 interface `ChatModel` | Go | M3-01 |
| M3-03 | Adapter embedding (OpenAI, Gemini, Cohere, Voyage, Jina) và rerank (Cohere, Jina, Voyage); interface `Embedder`, `Reranker` | Go | M3-01 |
| M3-04 | Gateway: timeout, retry có backoff khi 429/5xx, rate limit token bucket theo RPM/TPM của từng model, hàng ưu tiên chat > embed > Eval; đi ra ngoài qua egress proxy | Go | M3-02, M3-03 |
| M3-05 | Đếm token, tính chi phí mỗi lời gọi, ghi `model_usage`; kiểm ngân sách tháng, cảnh báo 80%, chặn 100% nếu bật | Go | M3-04 |
| M3-06 | API nội bộ `/internal/v1/embed`, `/internal/v1/chat` cho worker (service token, chỉ mạng nội bộ) + client Python | Go, Python | M3-04 |
| M3-07 | Model mặc định theo vai trò và resolve cho KB; Test connection; health check 5 phút | Go | M3-04 |
| M3-08 | Trang Settings → Cấu hình Model: thêm provider, model, test, chọn mặc định, xem chi phí theo ngày | Next.js | M3-07, M10-01 |

**Xong khi:** đổi model chat từ OpenAI sang Anthropic chỉ bằng UI, agent vẫn gọi tool đúng; provider trả 429 không làm mất job; chi phí trong `model_usage` khớp báo cáo của provider (sai lệch dưới 5%); worker không chứa API key nào.

## M4. Ingestion pipeline

Module lớn nhất và nằm trên đường găng: Go nhận file và điều phối, Python xử lý 4 stream `ingest.crawl`, `ingest.parse`, `ingest.chunk`, `ingest.embed`.

| Mã | Task | Stack | Phụ thuộc |
| --- | --- | --- | --- |
| M4-01 | API upload: cấp presigned URL MinIO (multipart, resume), kiểm quyền + quota, xác nhận hoàn tất → tạo `documents` (Queued); chống trùng SHA-256 | Go | M2-01 |
| M4-02 | Thư viện job chung: schema message, đẩy job có ưu tiên, bảng `pipeline_events`, publish trạng thái | Go, Python | M0-04 |
| M4-03 | Bước kiểm tra file: magic bytes, ClamAV, giới hạn giải nén; parser chạy trong container không ra mạng | Python | M4-02 |
| M4-04 | Parser PDF có bố cục (trang, heading, bảng); phát hiện trang không có text → `Needs OCR` / `Partially indexed` | Python | M4-03 |
| M4-05 | Parser DOCX (cây heading, bảng), PPTX (theo slide + speaker notes), XLSX (sheet → bảng) | Python | M4-03 |
| M4-06 | Crawler URL: 1 trang hoặc cả site (độ sâu, include/exclude, max trang), robots.txt, render JS khi cần, lọc boilerplate, chặn SSRF | Python | M4-02 |
| M4-07 | Chunker: theo heading, theo token cố định, theo slide/sheet (lặp header bảng); giữ vị trí (trang, offset) để trích dẫn; cấu hình lấy từ chunk\_config của KB | Python | M4-04, M4-05 |
| M4-08 | Phát hiện ngôn ngữ theo chunk (vi/en/zh); đếm token theo tokenizer của embedding model | Python | M4-07 |
| M4-09 | Embed theo batch qua model gateway (dense), tính sparse BM25 bằng tokenizer chung, upsert Qdrant với point ID = chunk\_id, payload đầy đủ; tạo collection + payload index | Python | M4-08, M4-13, M3-06 |
| M4-10 | Retry 3 lần có backoff, DLQ, API “Retry từ bước lỗi”; mã lỗi + thông điệp dễ hiểu | Go, Python | M4-02 |
| M4-11 | Xóa tài liệu (file + vector + chunk); thay phiên bản mới chỉ re-embed chunk đổi hash | Go, Python | M4-09 |
| M4-12 | SSE trạng thái realtime; UI upload kéo thả nhiều file, thêm URL, danh sách tài liệu với trạng thái, % tiến độ, lỗi | Go, Next.js | M4-01, M4-02 |
| M4-13 | Tokenizer BM25 chung cho vi/en/zh: đặc tả + bộ test vector dùng chung, cài đặt giống hệt ở Go (lúc truy vấn) và Python (lúc index) | Go, Python | M0-01 |

**Xong khi:** bộ 50 tài liệu mẫu khó (PDF nhiều bảng, XLSX nhiều sheet, PPTX, trang web JS, tài liệu tiếng Trung) đều về Indexed hoặc trạng thái đúng; PDF 100 trang dưới 2 phút; kill worker giữa chừng không tạo vector trùng.

Nên chuẩn bị bộ tài liệu mẫu này ngay tuần đầu; nó dùng lại cho test M4, M5 và Eval dataset của M8.

## M5. Retrieval & Agent

Toàn bộ viết bằng Go. Tách thành 2 lớp: `retrieval` (search + rerank, dùng chung cho agent, Inspector, Eval) và `agent` (vòng lặp + tool).

| Mã | Task | Stack | Phụ thuộc |
| --- | --- | --- | --- |
| M5-01 | Service retrieval: embed câu hỏi qua gateway (dense) + sparse BM25 bằng tokenizer chung (M4-13), truy vấn Qdrant hybrid, hợp nhất RRF; trả điểm từng nhánh | Go | M4-09, M4-13, M3-04 |
| M5-02 | Bảo vệ quyền trong retrieval: filter `kb_id IN AllowedKBIDs` luôn được chèn, không thể bỏ qua từ tham số gọi | Go | M5-01, M1-08 |
| M5-03 | Rerank top-k → top-n qua gateway (model reranker của provider) với ngưỡng điểm; cấu hình top-k/top-n theo KB | Go | M5-01 |
| M5-04 | Tool `list_kbs`, `search_kb`, `read_document` (đọc chunk lân cận); schema JSON cho function calling | Go | M5-03 |
| M5-05 | Agent loop ReAct: gọi model chat có tool, thực thi tool, giới hạn 5 lần gọi / 60 giây, dừng khi user hủy | Go | M5-04 |
| M5-06 | System prompt: trả lời theo ngôn ngữ câu hỏi, bắt buộc trích dẫn \[n\], từ chối khi thiếu ngữ cảnh; chunk bọc delimiter chống injection | Go | M5-05 |
| M5-07 | Hậu kiểm trích dẫn: \[n\] phải trỏ chunk đã truy xuất; map sang document, trang, offset | Go | M5-06 |
| M5-08 | Ghi trace: span cho rewrite, tool call, retrieval, rerank, LLM call (latency, token, model) | Go | M5-05, M0-06 |
| M5-09 | Stream sự kiện agent qua SSE: `step` (đang tìm gì), `token`, `citation`, `done`, `error` | Go | M5-05 |
| M5-10 | Fallback RAG đơn giản (1 lần retrieve) khi model không hỗ trợ tool calling hoặc tool call lỗi liên tiếp | Go | M5-05 |

**Xong khi:** test truy cập chéo KB và test injection đều pass; hybrid search + rerank p95 dưới 800 ms (gồm lời gọi provider) trên 1 triệu vector; hỏi tiếng Việt tìm được tài liệu tiếng Trung.

## M6. Chat

Trải nghiệm hỏi đáp đầy đủ (CHT-01 đến CHT-11) ngay Phase 1.

| Mã | Task | Stack | Phụ thuộc |
| --- | --- | --- | --- |
| M6-01 | API hội thoại: tạo, danh sách, tìm kiếm, đổi tên, xóa; lưu `messages`, `citations` | Go | M5-07 |
| M6-02 | Giao diện chat: khung hội thoại, streaming, nút dừng, render Markdown/bảng/code an toàn (sanitize), sao chép | Next.js | M5-09 |
| M6-03 | Chọn phạm vi: 1 KB, nhiều KB, hoặc để agent tự chọn | Next.js, Go | M6-02 |
| M6-04 | Hiển thị tiến trình agent theo sự kiện `step` | Next.js | M6-02 |
| M6-05 | Panel trích dẫn: mở file gốc (PDF viewer), cuộn tới trang, tô sáng đoạn; DOCX/XLSX/PPTX hiển thị text đã parse; URL mở trang gốc | Next.js, Go | M6-02, M5-07 |
| M6-06 | Sửa câu hỏi đã gửi, sinh lại câu trả lời; tự đặt tên hội thoại bằng model `utility` | Next.js, Go | M6-01 |
| M6-07 | Like/dislike kèm lý do; dislike tạo ticket cho Owner/Admin KB | Next.js, Go | M6-01 |
| M6-08 | Câu hỏi gợi ý khi mở KB và sau mỗi câu trả lời; xuất hội thoại ra Markdown/PDF kèm trích dẫn | Next.js, Go | M6-01 |
| M6-09 | Nút “Xem trace” trên mỗi câu trả lời (mở view của M7-06) | Next.js | M7-06 |

**Xong khi:** token đầu tiên p95 dưới 3 giây với 100 phiên đồng thời; bấm mọi trích dẫn đều mở đúng đoạn gốc.

## M7. Inspector & Trace

Phần lớn dữ liệu đã có sẵn từ M4 (`pipeline_events`, chunks, MinIO) và M5 (trace); M7 chủ yếu là API đọc và UI.

| Mã | Task | Stack | Phụ thuộc |
| --- | --- | --- | --- |
| M7-01 | Tab Timeline: các bước pipeline, thời gian, worker, model, retry, log/lỗi | Go, Next.js | M4-02 |
| M7-02 | Tab Parsed text: text đã parse cạnh file gốc, heading, bảng, trang bị bỏ qua | Go, Next.js | M4-04, M4-05 |
| M7-03 | Tab Chunks: danh sách chunk, số token, overlap tô màu, metadata, nhảy tới vị trí gốc; thống kê phân bố kích thước, chunk trùng | Go, Next.js | M4-07 |
| M7-04 | Tab Vectors: model, số chiều, point ID, payload, số sparse term, thời điểm upsert (đọc trực tiếp Qdrant) | Go, Next.js | M4-09 |
| M7-05 | Truy vấn thử: nhập câu hỏi, xem kết quả với điểm dense/sparse/RRF/rerank, thứ hạng trước và sau rerank | Go, Next.js | M5-03 |
| M7-06 | Chat Trace view: cây span, latency, token, chunk kèm điểm, prompt cuối (chỉ Owner/Admin KB) | Go, Next.js | M5-08 |
| M7-07 | Kiểm quyền cho Inspector/Trace theo ma trận; ghi audit khi xem trace của người khác | Go | M7-06, M9-03 |

**Xong khi:** với 1 câu trả lời sai bất kỳ, Owner KB truy ra được nguyên nhân (parse sai, chunk sai, hay retrieval sai) mà không cần đọc log server.

## M8. Eval

Eval chạy như job nền ưu tiên thấp, dùng lại service retrieval và agent của M5 để đo đúng hệ thống thật.

| Mã | Task | Stack | Phụ thuộc |
| --- | --- | --- | --- |
| M8-01 | Bảng và API dataset: cặp câu hỏi + đáp án chuẩn + chunk/tài liệu liên quan; nhập tay, import CSV/JSONL | Go | M2-01 |
| M8-02 | Sinh câu hỏi tự động từ mẫu chunk (vi/en/zh) bằng model chat; hàng chờ duyệt | Python | M8-01, M3-06 |
| M8-03 | Runner: chạy từng câu qua retrieval và agent, lưu snapshot cấu hình (chunk, model, top-k, prompt) | Go | M8-01, M5-05 |
| M8-04 | Chỉ số truy xuất: Hit@k, Recall@k, MRR, nDCG | Go | M8-03 |
| M8-05 | Chỉ số câu trả lời bằng LLM giám khảo: faithfulness, relevance, độ chính xác trích dẫn, tỉ lệ từ chối đúng | Python | M8-03 |
| M8-06 | UI Eval: quản lý dataset, chạy, xem kết quả từng câu, so sánh 2 lần chạy (tăng/giảm, câu đổi kết quả) | Next.js | M8-04, M8-05 |

**Xong khi:** chạy 200 câu trong dưới 30 phút mà chat không chậm đi; chạy lại cùng snapshot cho kết quả chênh dưới 2 điểm %.

## M9. Admin, quota & audit

| Mã | Task | Stack | Phụ thuộc |
| --- | --- | --- | --- |
| M9-01 | Quản lý user: danh sách, khóa/mở, gán System Admin, quyền tạo Workspace, chuyển Owner KB | Go, Next.js | M1-08 |
| M9-02 | Áp quota dung lượng, số tài liệu, token/ngày, ngân sách chi phí theo user/Workspace (bảng quotas, chỉnh trên Settings); rate limit API (Redis); trả 429 có thời gian thử lại | Go | M1-08 |
| M9-03 | Audit log chỉ ghi thêm: đăng nhập, đổi quyền, upload/xóa, đổi cấu hình, xem trace | Go | M0-05 |
| M9-04 | UI audit log: lọc theo người, hành động, thời gian; xuất CSV | Next.js | M9-03 |

**Xong khi:** mọi hành động trong danh sách ADM-03 đều có bản ghi audit; vượt quota bị chặn với thông báo rõ ràng.

## M10. Settings

Settings là một khu giao diện gom các trang cấu hình, không có bảng hay engine cấu hình riêng: mỗi trang gọi API của module sở hữu dữ liệu. Trang Cấu hình Model (M3-08) và cấu hình KB (M2-07) cũng nằm trong khu này.

| Mã | Task | Stack | Phụ thuộc |
| --- | --- | --- | --- |
| M10-01 | Khung Settings: layout, menu trái theo nhóm (Hồ sơ, Thành viên, Cấu hình Model, Knowledge Base, Quota & chi phí, Audit log), chỉ hiện mục user có quyền | Next.js | M1-09 |
| M10-02 | Hồ sơ cá nhân: tên, ảnh đại diện, đổi mật khẩu, ngôn ngữ, giao diện sáng/tối; API `GET, PATCH /me`, `POST /me/password` | Next.js, Go | M10-01, M1-06 |
| M10-03 | Thành viên: danh sách thành viên workspace/KB kèm role, mời, đổi role, gỡ; System Admin xem mọi user, khóa/mở (dùng API M2-02, M9-01) | Next.js | M10-01, M2-02, M9-01 |
| M10-04 | Đưa các trang quản trị vào Settings: Knowledge Base (M2-07), Cấu hình Model (M3-08), Quota & chi phí (M9-02, M3-05), Audit log (M9-04) | Next.js | M10-01 |

**Xong khi:** mỗi role chỉ thấy đúng các mục được phép (Viewer: Hồ sơ; Admin KB: thêm Thành viên, Knowledge Base; System Admin: tất cả); đổi model mặc định hoặc role thành viên có hiệu lực ngay và có bản ghi audit.
