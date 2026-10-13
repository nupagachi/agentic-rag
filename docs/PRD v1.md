# PRD v1 — Agentic RAG nội bộ

Oct 9, 2026 · @Nhu Canh

## 1. Tổng quan

Xây dựng nền tảng Agentic RAG triển khai on-premise cho nội bộ: người dùng tạo Knowledge Base (KB), nạp tài liệu và URL, theo dõi từng bước xử lý, rồi hỏi đáp với agent có trích dẫn nguồn. Ứng dụng và toàn bộ dữ liệu lưu trên hạ tầng công ty; các model (chat, embedding, rerank, OCR) được gọi qua LLM provider bên ngoài, không chạy model local. Mọi trang cấu hình nằm trong một khu Settings chung trên giao diện.

**Vấn đề hiện tại**

- Tài liệu nội bộ (quy trình, hợp đồng, tài liệu kỹ thuật, báo cáo) nằm rải rác, khó tìm, đa ngôn ngữ (Việt, Anh, Trung).
- Chatbot SaaS công cộng không phân quyền theo phòng ban, không lưu tài liệu và lịch sử trong công ty, không chọn được provider theo hợp đồng của công ty.
- Các thử nghiệm RAG trước đó là hộp đen: không biết tài liệu được chunk thế nào, vì sao câu trả lời sai, không đo được chất lượng.

**Mục tiêu sản phẩm**

1. Hỏi đáp chính xác trên tài liệu nội bộ, mọi câu trả lời có trích dẫn bấm được về đúng đoạn gốc.
2. Bảo mật mặc định: RBAC 2 tầng, cô lập dữ liệu ngay ở tầng truy vấn vector; kiểm soát được dữ liệu nào được gửi tới provider nào.
3. Minh bạch toàn bộ pipeline: xem được parse, chunk, embed, upsert của từng tài liệu và trace của từng câu trả lời.
4. Đo được chất lượng bằng Eval, so sánh trước và sau mỗi lần đổi cấu hình hoặc đổi model.
5. Chịu tải cao, scale ngang từng thành phần độc lập.
6. Không khóa vào một provider: mỗi vai trò model (chat, embedding, rerank, utility, OCR) chọn provider và model riêng, đổi được trên giao diện.
7. Quản trị tập trung: một khu Settings trên giao diện gom hồ sơ, thành viên, cấu hình model, KB, quota; kiểm soát được chi phí model.

**Ngoài phạm vi (non-goals)**

- Không phải sản phẩm SaaS bán ra ngoài, không cần multi-tenant giữa các công ty.
- Không tự host, huấn luyện hay fine-tune model; không cần GPU.
- Không thay thế hệ thống quản lý tài liệu (DMS); KB chỉ là bản sao để truy xuất.
- OCR, đọc ảnh và đăng nhập Google không có ở Phase 1 (đưa sang Phase 2).

## 2. Phạm vi và lộ trình

Sản phẩm được làm hoàn thiện qua 3 phase; Phase 1 đã đủ để cả công ty dùng thật. Ngày cụ thể sẽ gắn khi có quy mô team.

&#91;embedded content: lộ trình · 3 phase, 2 gate\]

Quy ước ưu tiên trong các bảng yêu cầu: P0 thuộc Phase 1, P1 thuộc Phase 2, P2 thuộc Phase 3. Riêng Chat được làm đủ ngay Phase 1 theo yêu cầu. Qua mỗi gate phải đạt tiêu chí ghi dưới gate.

Phân rã P0 thành module và task: Phân rã P0

## 3. Persona và phân quyền

Phân quyền chia 2 tầng: tầng hệ thống (System Admin, User) và tầng Workspace/KB với 4 role Owner, Admin, Contributor, Viewer. Quyền được kiểm tra tại Go API và áp vào filter Qdrant, không chỉ ẩn trên UI.

**Persona**

| Persona | Vai trò điển hình | Nhu cầu chính |
| --- | --- | --- |
| System Admin (IT/Platform) | System Admin | Cấu hình provider và Settings hệ thống, quản lý user, quota, theo dõi tải và chi phí model, audit |
| Trưởng nhóm tri thức (HR, Pháp chế, IT Helpdesk) | Owner / Admin của KB | Dựng KB, chỉnh chunking và prompt, xem trace, chạy Eval |
| Người đóng góp tài liệu | Contributor | Upload tài liệu/URL, theo dõi trạng thái, sửa chunk của mình |
| Nhân viên hỏi đáp | Viewer | Hỏi nhanh, có nguồn để kiểm chứng |

**Tầng hệ thống**

- **System Admin:** quản lý user (khóa, mở, reset), Settings hệ thống (LLM provider, model mặc định, quota, ngân sách, chính sách), audit log toàn hệ thống. Không tự động đọc được nội dung KB; muốn vào phải được thêm làm thành viên (có ghi audit).
- **User:** tạo Workspace/KB (nếu được cấp quyền tạo), tham gia KB được mời, chỉnh Settings cá nhân.

**Tầng Workspace/KB**

Workspace (theo phòng ban/team) chứa nhiều KB. Role gán ở Workspace được kế thừa xuống mọi KB bên trong; có thể ghi đè ở từng KB. Gán quyền cho user hoặc group.

| Quyền | Owner | Admin | Contributor | Viewer |
| --- | --- | --- | --- | --- |
| Xóa KB, chuyển quyền sở hữu | Có | Không | Không | Không |
| Cấu hình KB (chunking, model, prompt agent, từ điển) | Có | Có | Không | Không |
| Mời thành viên, đổi role | Có | Tới Contributor | Không | Không |
| Upload tài liệu, thêm URL | Có | Có | Có | Không |
| Xóa, re-index, sửa chunk | Có | Có | Chỉ tài liệu của mình | Không |
| Xem tài liệu, trạng thái, chunk | Có | Có | Có | Có |
| Chat với KB | Có | Có | Có | Có |
| Xem Chat Trace của người khác | Có | Có | Không | Không |
| Quản lý Eval dataset, chạy Eval | Có | Có | Không | Không |
| Xem thống kê sử dụng KB | Có | Có | Không | Không |

**Quy tắc**

- Mỗi KB luôn có đúng 1 Owner; Owner rời công ty thì System Admin chuyển quyền.
- Người dùng chỉ xem trace của chính hội thoại mình; Owner/Admin xem được trace của mọi hội thoại trong KB.
- Thu hồi quyền có hiệu lực trong tối đa 1 phút (cache quyền trong Redis với TTL ngắn).

## 4. Yêu cầu chức năng: nền tảng và nạp dữ liệu

Mỗi yêu cầu có mã để truy vết sang task và test. Độ ưu tiên: P0 = Phase 1, P1 = Phase 2, P2 = Phase 3.

### 4.1 Xác thực (AUTH)

| Mã | Yêu cầu | Ưu tiên |
| --- | --- | --- |
| AUTH-01 | Đăng ký bằng email + mật khẩu, xác thực email; cấu hình chọn đăng ký tự do (giới hạn domain email công ty) hoặc chỉ qua lời mời của Admin | P0 |
| AUTH-02 | Đăng nhập trả access token JWT (mặc định 15 phút) + refresh token (mặc định 7 ngày, lưu hash trong Postgres); thời hạn cấu hình được | P0 |
| AUTH-03 | Refresh token xoay vòng (rotation); phát hiện dùng lại token cũ thì thu hồi cả chuỗi phiên | P0 |
| AUTH-05 | Quên / đổi mật khẩu; hash argon2id; chính sách mật khẩu cấu hình được (mặc định tối thiểu 10 ký tự) | P0 |
| AUTH-06 | Khóa tạm tài khoản sau 5 lần sai trong 15 phút (cấu hình được); rate limit endpoint đăng nhập | P0 |
| AUTH-04 | Đăng nhập Google OAuth 2.0 / OIDC; bật/tắt và giới hạn domain Google Workspace trong Settings; liên kết với tài khoản email cùng địa chỉ | P1 |
| AUTH-07 | Quản lý phiên: xem thiết bị đang đăng nhập, đăng xuất từng phiên hoặc tất cả | P1 |
| AUTH-08 | Xác thực 2 bước (TOTP) | P1 |
| AUTH-09 | API key cá nhân có phạm vi (scope) theo KB, có hạn dùng | P1 |
| AUTH-10 | LDAP/AD hoặc SSO OIDC nội bộ (Keycloak) | P2 |

### 4.2 Workspace và Knowledge Base (KB)

| Mã | Yêu cầu | Ưu tiên |
| --- | --- | --- |
| KB-01 | Tạo, sửa, xóa Workspace và KB; KB có tên, mô tả (agent dùng mô tả để chọn KB), ngôn ngữ chính, tag | P0 |
| KB-02 | Mời thành viên, gán role theo ma trận mục 3; gán theo group | P0 |
| KB-03 | Cấu hình chunking theo KB: chiến lược (theo cấu trúc heading, cố định token, theo slide/sheet), kích thước, overlap | P0 |
| KB-04 | Chọn model embedding/chat/rerank cho KB (mặc định lấy từ hệ thống); embedding bị khóa sau lần index đầu | P0 |
| KB-05 | Đổi embedding model hoặc cấu hình chunk → job re-index nền, chuyển generation khi xong, không gián đoạn chat | P0 |
| KB-06 | Xóa KB: xóa mềm 30 ngày, sau đó xóa hẳn file MinIO, vector Qdrant, bản ghi Postgres | P0 |
| KB-07 | Cấu hình agent theo KB: system prompt, giọng văn, câu chào, câu hỏi gợi ý | P1 |
| KB-08 | Từ điển thuật ngữ / từ viết tắt theo KB, dùng để mở rộng câu hỏi | P1 |
| KB-09 | Nhãn phân loại dữ liệu (Public / Internal / Confidential) cho tài liệu | P1 |
| KB-10 | Backup / restore 1 KB (metadata + vector + file) | P1 |
| KB-11 | KB template (HR, Pháp chế, Kỹ thuật) kèm preset chunking và prompt | P2 |

### 4.3 Nạp dữ liệu và pipeline (ING)

Nguồn Phase 1: nhóm **Document** (PDF có lớp text, DOCX, XLSX, PPTX) và nhóm **URL** (mọi URL HTTP/HTTPS truy cập được). Ảnh và PDF scan được nhận nhưng chờ OCR ở Phase 2.

| Mã | Yêu cầu | Ưu tiên |
| --- | --- | --- |
| ING-01 | Upload nhiều file cùng lúc (kéo thả, tối đa 100 file/lượt, 200 MB/file, cấu hình được) qua presigned URL MinIO, hỗ trợ resume | P0 |
| ING-02 | Kiểm tra loại file theo magic bytes (không tin đười dẫn), quét malware (ClamAV) trước khi parse | P0 |
| ING-03 | Chống trùng theo SHA-256: cùng nội dung trong KB thì báo và bỏ qua | P0 |
| ING-04 | Parse có nhận diện bố cục: PDF giữ số trang, heading, bảng; DOCX giữ cây heading; XLSX mỗi sheet thành bảng, chunk theo nhóm dòng có lặp header; PPTX theo slide kèm speaker notes | P0 |
| ING-05 | URL: crawl 1 trang hoặc cả site (độ sâu, include/exclude pattern, tối đa số trang), tôn trọng robots.txt, render JS khi cần, lọc boilerplate (menu, footer) | P0 |
| ING-06 | Phát hiện ngôn ngữ theo chunk (vi / en / zh), lưu vào payload | P0 |
| ING-07 | Trang PDF không có text hoặc file ảnh → trạng thái `Needs OCR` (toàn bộ) hoặc `Partially indexed` kèm danh sách trang bị bỏ qua | P0 |
| ING-08 | Embed theo batch, dense vector qua embedding model của provider (gọi qua model gateway) và sparse vector BM25 tính tại chỗ, upsert Qdrant kèm payload: kb\_id, doc\_id, chunk\_id, trang, heading, ngôn ngữ, nhãn phân loại | P0 |
| ING-09 | Trạng thái thời gian thực qua SSE (xem vòng đời bên dưới), % tiến độ, số trang/chunk đã xử lý | P0 |
| ING-10 | Lỗi có mã + lý do dễ hiểu; Retry từ bước lỗi; tự retry 3 lần với backoff, sau đó vào dead-letter queue | P0 |
| ING-11 | Xóa tài liệu xóa luôn vector và file; thay phiên bản mới chỉ re-embed chunk thay đổi (so hash chunk) | P0 |
| ING-12 | Thông báo khi index xong hoặc lỗi (in-app; email tùy chọn) | P1 |
| ING-13 | Re-crawl URL định kỳ (hằng ngày/tuần), chỉ re-index trang đổi nội dung | P1 |
| ING-14 | Sửa, tách, gộp, vô hiệu hóa chunk thủ công (re-embed chunk đó) | P1 |
| ING-15 | Che PII (CCCD, SĐT, email) tùy chọn trước khi embed | P1 |
| ING-16 | OCR (vi/en/zh) cho ảnh và PDF scan qua provider (model vision hoặc OCR API), nút "Process pending OCR" | P1 |
| ING-17 | Vision: mô tả hình, biểu đồ trong PDF/PPTX bằng model vision của provider | P2 |
| ING-18 | Connector đồng bộ: Google Drive, Confluence, SharePoint; URL nội bộ cần đăng nhập | P2 |

**Vòng đời tài liệu**: `Queued → Parsing → Chunking → Embedding → Indexing → Indexed`. Nhánh phụ: `Failed` (kèm bước lỗi), `Needs OCR`, `Partially indexed`, `Re-indexing`, `Deleting`. Mỗi lần chuyển trạng thái ghi 1 sự kiện vào bảng `pipeline_events` để Inspector hiển thị timeline.

**Tiêu chí nghiệm thu chính**

- PDF 100 trang có text: từ Queued tới Indexed dưới 2 phút khi hệ thống không tải.
- Trạng thái trên UI cập nhật chậm tối đa 2 giây so với worker.
- Kill worker giữa chừng: job được worker khác nhận lại, không tạo vector trùng (upsert theo chunk\_id xác định).
- Xóa tài liệu: sau 1 phút không còn chunk nào của nó xuất hiện trong kết quả tìm kiếm.

### 4.4 Kết nối LLM Provider (MDL)

Không chạy model local. Mọi model là một model của provider bên ngoài, khai báo trong Settings và gọi qua **model gateway** nằm trong Go: worker Python cũng gọi qua gateway này, nên API key chỉ nằm ở Go.

| Mã | Yêu cầu | Ưu tiên |
| --- | --- | --- |
| MDL-01 | Khai báo Provider: OpenAI, Azure OpenAI, Anthropic, Google Gemini, OpenAI-compatible (gateway nội bộ, OpenRouter…); cho embedding và rerank thêm Cohere, Voyage AI, Jina AI. Trường: base URL, API key (mã hóa AES-256-GCM), org/project, region, timeout | P0 |
| MDL-02 | Khai báo Model thuộc provider, gán vai trò `chat`, `embedding`, `reranker`, `utility` (`ocr`, `vision` ở P1); thuộc tính: context window, số chiều (embedding), hỗ trợ tool calling, giá / 1 triệu token vào-ra, giới hạn RPM/TPM | P0 |
| MDL-03 | Model gateway: chuẩn hóa API của các provider (chat stream + tool calling, embed, rerank); timeout, retry có backoff khi 429/5xx, rate limit theo provider/model, đếm token và tính chi phí mỗi lời gọi | P0 |
| MDL-04 | Model mặc định toàn hệ thống cho mỗi vai trò; Workspace/KB ghi đè được | P0 |
| MDL-05 | Nút Test connection; health check định kỳ (5 phút, lời gọi nhỏ); provider/model lỗi hiển thị cảnh báo | P0 |
| MDL-06 | Theo dõi chi phí theo provider, model, workspace, KB, ngày; ngân sách tháng: cảnh báo ở 80%, tùy chọn chặn ở 100% | P0 |
| MDL-07 | Model dự phòng (fallback) cho `chat` và `reranker` khi model chính lỗi, timeout hoặc bị rate limit | P1 |
| MDL-08 | Playground: thử prompt, thử embed 2 câu xem cosine, thử rerank | P1 |
| MDL-09 | Chính sách dữ liệu theo provider: đánh dấu provider được phép nhận dữ liệu Confidential; KB Confidential chỉ chọn được provider đó | P1 |

Quy tắc: API key không bao giờ trả về giao diện (chỉ hiện 4 ký tự cuối); không cho xóa model đang là embedding của bất kỳ KB nào; đổi embedding của KB luôn hiện cảnh báo re-index kèm ước tính thời gian và chi phí embed. Model embedding phải hỗ trợ đa ngôn ngữ (tối thiểu vi, en, zh); chọn qua Eval.

### 4.5 Settings chung (SET)

Một khu **Settings** trên giao diện gom mọi trang cấu hình về một chỗ, menu bên trái chỉ hiện mục người dùng có quyền. Settings chỉ là nơi đặt giao diện: dữ liệu vẫn nằm ở bảng của từng chức năng (user, thành viên, model, KB, quota), không có bảng settings riêng.

| Mục trong Settings | Ai thấy | Nội dung | Ưu tiên |
| --- | --- | --- | --- |
| Hồ sơ cá nhân (User Profile) | Mọi user | Tên, ảnh đại diện, email (chỉ xem), đổi mật khẩu, ngôn ngữ, giao diện sáng/tối | P0 |
| Thành viên (Members) | Owner/Admin của workspace, KB; System Admin xem mọi user | Xem danh sách thành viên và role, mời, đổi role, gỡ; System Admin khóa/mở tài khoản | P0 |
| Cấu hình Model | System Admin (Admin KB chỉ xem) | Provider, API key, model theo vai trò, model mặc định, test kết nối (mục 4.4) | P0 |
| Knowledge Base | Owner/Admin của KB | Thông tin chung, chunking, model riêng cho KB, prompt agent | P0 |
| Quota và chi phí | System Admin | Quota dung lượng, số tài liệu, token/ngày; ngân sách tháng; báo cáo chi phí theo provider, workspace, KB | P0 |
| Audit log | System Admin | Lọc theo người, hành động, thời gian; xuất CSV | P0 |
| Bảo mật và đăng nhập | System Admin | Chế độ đăng ký, domain email, chính sách mật khẩu, thời hạn phiên, Google OAuth, 2FA | P1 |
| Thông báo | Mọi user | Kênh nhận, sự kiện muốn nhận | P1 |
| Phiên và API key | Mọi user | Thiết bị đang đăng nhập, API key cá nhân | P1 |

Quy tắc chung: thay đổi có hiệu lực ngay không cần khởi động lại; mọi thay đổi cấu hình (model, quota, role) ghi audit log. Ở Phase 1, các giá trị bảo mật (chế độ đăng ký, chính sách mật khẩu, giới hạn file) lấy từ cấu hình triển khai (`.env`); trang chỉnh trên giao diện có ở Phase 2.

## 5. Yêu cầu chức năng: Agent và Chat

Agent chạy trong Go, vòng lặp ReAct với function calling qua API chuẩn OpenAI: lập kế hoạch, gọi tool, đánh giá kết quả, tìm lại nếu chưa đủ, rồi trả lời có trích dẫn. Chat làm đủ P0 và P1 ngay Phase 1.

### 5.1 Agent và truy xuất (AGT)

| Mã | Yêu cầu | Ưu tiên |
| --- | --- | --- |
| AGT-01 | Hybrid search trên Qdrant: dense (embedding của provider) + sparse (BM25), hợp nhất bằng RRF; top-k cấu hình theo KB (mặc định 30 → rerank còn 8) | P0 |
| AGT-02 | Rerank bằng model `reranker` (model rerank đa ngôn ngữ của provider, ví dụ Cohere Rerank, Jina Reranker); ngưỡng điểm tối thiểu, dưới ngưỡng coi là không tìm thấy | P0 |
| AGT-03 | Bộ tool: `list_kbs` (KB user được phép + mô tả), `search_kb(kb_ids, query, filters)`, `read_document(doc_id, range)` (đọc chunk lân cận để đủ ngữ cảnh) | P0 |
| AGT-04 | Bắt buộc filter `kb_id IN (KB user có quyền)` ở tầng Go cho mọi truy vấn Qdrant; LLM không thể truyền kb\_id ngoài danh sách | P0 |
| AGT-05 | Giới hạn vòng lặp: tối đa 5 lần gọi tool, tổng thời gian 60 giây; hết giới hạn thì trả lời với những gì đã có và nói rõ | P0 |
| AGT-06 | Trích dẫn bắt buộc: mỗi ý trả lời gắn \[n\] trỏ tới chunk; hậu kiểm trích dẫn trỏ đúng chunk đã truy xuất | P0 |
| AGT-07 | Không có ngữ cảnh đủ → trả lời "không tìm thấy trong tài liệu" và gợi ý KB/từ khóa khác, không bịa | P0 |
| AGT-08 | Trả lời theo ngôn ngữ câu hỏi; hỗ trợ hỏi xuyên ngôn ngữ (hỏi tiếng Việt, nguồn tiếng Trung), trích dẫn giữ nguyên văn gốc | P0 |
| AGT-09 | Chống prompt injection từ nội dung tài liệu: nội dung chunk bọc trong delimiter, system prompt cấm làm theo lệnh trong tài liệu, tool không có hành động ghi | P0 |
| AGT-10 | Viết lại câu hỏi theo ngữ cảnh hội thoại ("còn cái kia thì sao?"), tách câu hỏi phức tạp thành câu con, mở rộng bằng từ điển KB | P1 |
| AGT-11 | Lọc theo metadata trong câu hỏi (tên tài liệu, ngày, tag, ngôn ngữ) | P1 |
| AGT-12 | Chunk nhãn Confidential chỉ trả cho role đủ quyền; có thể ép dùng model riêng | P1 |
| AGT-13 | Semantic cache theo KB (câu hỏi gần giống, cùng quyền, TTL 24 giờ; tự xóa khi KB thay đổi) | P1 |
| AGT-14 | Tool mở rộng: tóm tắt cả tài liệu, so sánh 2 tài liệu, truy vấn bảng XLSX có cấu trúc | P2 |
| AGT-15 | Public API (OpenAI-compatible) và MCP server để hệ thống nội bộ khác gọi KB | P2 |

### 5.2 Trải nghiệm Chat (CHT)

| Mã | Yêu cầu | Ưu tiên |
| --- | --- | --- |
| CHT-01 | Streaming câu trả lời qua SSE; nút dừng sinh | P0 |
| CHT-02 | Chọn phạm vi: 1 KB, nhiều KB, hoặc để agent tự chọn trong các KB được phép | P0 |
| CHT-03 | Lịch sử hội thoại: danh sách, tìm kiếm, đổi tên (tự đặt tên bằng model `utility`), xóa | P0 |
| CHT-04 | Trích dẫn bấm được: mở panel bên phải hiển thị file gốc, cuộn tới trang và tô sáng đoạn; URL mở trang gốc | P0 |
| CHT-05 | Hiển thị tiến trình agent theo thời gian thực: đang tìm gì, ở KB nào, tìm thấy bao nhiêu đoạn | P0 |
| CHT-06 | Render Markdown, bảng, code block; sao chép câu trả lời | P0 |
| CHT-07 | Sửa câu hỏi đã gửi, sinh lại câu trả lời | P0 |
| CHT-08 | Like / dislike kèm lý do (sai, thiếu, trích dẫn sai, không liên quan); dislike tạo ticket cho Owner/Admin KB | P0 |
| CHT-09 | Câu hỏi gợi ý: khi mở KB và sau mỗi câu trả lời | P0 |
| CHT-10 | Xuất hội thoại ra Markdown / PDF kèm trích dẫn | P0 |
| CHT-11 | Nút "Xem trace" trên mỗi câu trả lời (xem mục 6.2) | P0 |
| CHT-12 | Chia sẻ hội thoại cho người có quyền trên KB tương ứng | P2 |

**Tiêu chí nghiệm thu chính**

- Token đầu tiên hiển thị dưới 3 giây (p95) ở tải thiết kế.
- 100% câu trả lời có nội dung từ tài liệu đều có ít nhất 1 trích dẫn hợp lệ.
- Test bảo mật: user A hỏi về nội dung chỉ có trong KB của user B → agent không trả ra chunk nào của KB B, kể cả khi prompt cố ý yêu cầu.
- Tài liệu chứa lệnh "ignore previous instructions" không làm agent đổi hành vi (có bộ test injection trong Eval).

## 6. Yêu cầu chức năng: Quan sát, đánh giá và quản trị

Mọi bước từ upload tới câu trả lời đều xem được và đo được. Inspector và Eval là P0, vì đây là cách duy nhất để tin cậy và cải thiện chất lượng RAG.

### 6.1 Document Inspector (INS)

| Mã | Yêu cầu | Ưu tiên |
| --- | --- | --- |
| INS-01 | Tab Timeline: từng bước pipeline với thời gian bắt đầu/kết thúc, worker xử lý, model dùng, số lần retry, log và lỗi | P0 |
| INS-02 | Tab Parsed text: văn bản trích xuất đặt cạnh file gốc, hiển thị heading, bảng, số trang; trang bị bỏ qua (cần OCR) được đánh dấu | P0 |
| INS-03 | Tab Chunks: danh sách chunk gồm số token, vùng overlap tô màu, metadata (trang, heading, ngôn ngữ); bấm chunk nhảy tới vị trí trong file gốc | P0 |
| INS-04 | Thống kê chunk: phân bố kích thước, chunk quá ngắn/quá dài, chunk trùng lặp | P0 |
| INS-05 | Tab Vectors: embedding model, số chiều, point ID Qdrant, payload, số sparse term, thời điểm upsert | P0 |
| INS-06 | Truy vấn thử: nhập câu hỏi, xem chunk nào của tài liệu/KB được trả về kèm điểm dense, sparse, RRF và rerank, thứ hạng trước/sau rerank | P0 |
| INS-07 | Xem trước chunking: chạy thử 2 cấu hình chunk trên cùng tài liệu, so sánh song song trước khi áp dụng | P1 |
| INS-08 | Pipeline dashboard theo KB: số tài liệu theo trạng thái, hàng đợi, thời gian xử lý trung bình mỗi bước, lỗi phổ biến | P1 |

### 6.2 Chat Trace (TRC)

| Mã | Yêu cầu | Ưu tiên |
| --- | --- | --- |
| TRC-01 | Mỗi câu trả lời lưu 1 trace gồm các span: rewrite câu hỏi, từng lần gọi tool (tham số, kết quả), retrieval, rerank, lời gọi LLM | P0 |
| TRC-02 | Mỗi span có độ trễ, token vào/ra, model dùng; chunk kèm điểm trước/sau rerank | P0 |
| TRC-03 | Xem prompt cuối cùng gửi LLM (Owner/Admin KB) | P0 |
| TRC-04 | Theo dõi theo chuẩn OpenTelemetry, trace\_id dẫn xuyên Go → model server | P0 |
| TRC-05 | Từ trace, “Thêm vào Eval dataset” với đáp án chuẩn do người dùng sửa | P1 |
| TRC-06 | Lưu trace 90 ngày (cấu hình được), sau đó chỉ giữ số liệu tổng hợp | P1 |

### 6.3 Eval (EVL)

| Mã | Yêu cầu | Ưu tiên |
| --- | --- | --- |
| EVL-01 | Eval dataset theo KB: cặp câu hỏi + đáp án chuẩn + chunk/tài liệu liên quan; nhập tay, import CSV/JSONL | P0 |
| EVL-02 | Sinh dataset tự động: model `chat` sinh câu hỏi từ mẫu chunk (vi/en/zh); người duyệt trước khi dùng | P0 |
| EVL-03 | Chỉ số truy xuất: Hit@k, Recall@k, MRR, nDCG | P0 |
| EVL-04 | Chỉ số câu trả lời (LLM làm giám khảo): faithfulness, answer relevance, độ chính xác trích dẫn, tỉ lệ từ chối đúng với câu không có trong tài liệu | P0 |
| EVL-05 | Mỗi lần chạy lưu snapshot cấu hình (chunk, model, top-k, prompt) để tái lập | P0 |
| EVL-06 | So sánh 2 lần chạy: chỉ số tăng/giảm, danh sách câu hỏi đổi kết quả, xem chi tiết từng câu | P0 |
| EVL-07 | Bộ test bảo mật có sẵn: prompt injection, truy cập chéo KB | P1 |
| EVL-08 | Tự chạy Eval sau mỗi lần đổi cấu hình KB hoặc model; cảnh báo khi chỉ số giảm quá ngưỡng (mặc định 5 điểm %) | P1 |
| EVL-09 | Dislike trong chat tự vào hàng chờ “cần gán nhãn” để bổ sung dataset | P1 |
| EVL-10 | A/B test 2 cấu hình retrieval trên lưu lượng thật | P2 |

Eval chạy như job nền ưu tiên thấp, không làm chậm chat của người dùng.

### 6.4 Quản trị và quota (ADM)

| Mã | Yêu cầu | Ưu tiên |
| --- | --- | --- |
| ADM-01 | Quản lý user: danh sách, khóa/mở, gán System Admin, quyền tạo Workspace | P0 |
| ADM-02 | Quota theo user/Workspace: dung lượng, số tài liệu, token/ngày, ngân sách chi phí model; rate limit API theo user (giá trị chỉnh trong Settings) | P0 |
| ADM-03 | Audit log: đăng nhập, đổi quyền, upload/xóa tài liệu, đổi cấu hình, xem trace; chỉ ghi thêm, không sửa; lọc và xuất CSV | P0 |
| ADM-04 | Dashboard sử dụng: user hoạt động, số câu hỏi, token theo KB/model, tỉ lệ “không tìm thấy”, tỉ lệ dislike | P1 |
| ADM-05 | Giám sát hạ tầng: Prometheus + Grafana cho service, hàng đợi Redis, Qdrant; độ trễ, lỗi và 429 theo provider | P1 |
| ADM-06 | Dashboard chi phí model theo provider, workspace, KB; cấu hình toàn cục nằm trong Settings (mục 4.5) | P1 |

## 7. Yêu cầu phi chức năng

Mục tiêu thiết kế: 500 user, 100 user đồng thời, 100.000 tài liệu, token đầu tiên dưới 3 giây; mọi thành phần scale ngang để vượt mức này khi cần.

### 7.1 Hiệu năng và tải

| Chỉ tiêu | Mục tiêu |
| --- | --- |
| Token đầu tiên (chat) | p95 dưới 3 giây |
| Hybrid search + rerank | p95 dưới 800 ms (gồm lời gọi embed và rerank tới provider) |
| API CRUD (Go) | p95 dưới 200 ms |
| Chat đồng thời | 100 phiên streaming không suy giảm; 300 phiên vẫn hoạt động (giới hạn bởi rate limit của provider) |
| Nạp tài liệu | Tối thiểu 2.000 trang/giờ; trần phụ thuộc rate limit embedding (TPM) của provider |
| Qdrant | 100.000 tài liệu \~ 10–20 triệu vector, truy vấn p95 dưới 100 ms |
| Khả dụng | 99,5% giờ làm việc; không có điểm lỗi đơn cho API, Postgres, Redis, Qdrant |

Nguyên tắc chịu tải:

- Go API và Python worker stateless, scale ngang bằng cách tăng số container (`docker compose --scale`) theo CPU và độ dài hàng đợi.
- Mỗi bước pipeline có nhóm worker riêng; bước embed bị giới hạn bởi rate limit của provider nên không được chặn parse/crawl.
- Hàng đợi có ưu tiên: file nhỏ và người dùng đang chờ trước, import hàng loạt và Eval sau.
- Model gateway chia ngân sách rate limit của provider: chat của người dùng được ưu tiên hơn embed nền và Eval.
- Backpressure: provider trả 429 thì gateway giảm tốc và worker lấy job chậm lại; chat nhận thông báo rõ ràng thay vì treo.
- Connection pool cho Postgres, Qdrant gRPC; HTTP keep-alive tới provider.

### 7.2 Bảo mật

- **Truyền tải:** TLS 1.2+ cho mọi kết nối, kể cả nội bộ giữa service (mTLS khuyến nghị).
- **Lưu trữ:** MinIO bật server-side encryption; Postgres mã hóa đĩa; secret (API key model, OAuth secret) mã hóa AES-256-GCM, khóa chính nằm ngoài DB (Docker secrets hoặc Vault).
- **Cô lập dữ liệu:** mọi truy vấn Postgres theo workspace/KB qua middleware kiểm quyền; mọi truy vấn Qdrant bắt buộc filter kb\_id; file MinIO chỉ truy cập qua presigned URL hạn 5 phút sau khi kiểm quyền.
- **Ứng dụng web:** chống CSRF, CSP chặt, refresh token trong cookie httpOnly + Secure + SameSite=Strict, sanitize Markdown khi render.
- **File đầu vào:** kiểm magic bytes, ClamAV, giới hạn kích thước giải nén (chống zip bomb), parser chạy trong container không có quyền mạng ra ngoài.
- **Crawler:** chặn SSRF — không truy cập IP nội bộ/metadata (169.254.x.x, dải private) trừ danh sách domain được System Admin cho phép.
- **LLM:** chống prompt injection (AGT-09); tool của agent chỉ đọc; không ghi nội dung tài liệu vào log ứng dụng.
- **Kiểm thử:** quét SAST/dependency trong CI; pentest nội bộ trước khi go-live.

* **Dữ liệu gửi tới provider:** nội dung chunk (lúc embed), câu hỏi và ngữ cảnh (lúc chat, rerank) rời khỏi công ty. Chỉ dùng provider có hợp đồng doanh nghiệp: không dùng dữ liệu để huấn luyện, thời gian lưu tối thiểu (zero data retention nếu có), region phù hợp. Mọi lưu lượng ra ngoài đi qua egress proxy có log; chỉ mở tới domain của provider đã khai báo. Che PII (P1) và chính sách provider cho KB Confidential (MDL-09) giảm rủi ro thêm.

### 7.3 Ngôn ngữ và giao diện

- Nội dung: tiếng Việt, tiếng Anh, tiếng Trung (giản thể và phồn thể).
- Giao diện: tiếng Việt và tiếng Anh (i18n sẵn để thêm tiếng Trung).
- Hỗ trợ desktop là chính; chat dùng được trên mobile. Chế độ sáng/tối.

### 7.4 Vận hành

- Triển khai bằng Docker Compose trên 2 máy chủ (ứng dụng, dữ liệu); chuyển sang Kubernetes khi cần HA nhiều node. Không cần GPU.
- Cần kết nối HTTPS ra ngoài tới provider qua egress proxy; image lưu trong registry nội bộ.
- Backup: Postgres hằng ngày + WAL, snapshot Qdrant hằng ngày, MinIO versioning; RPO 24 giờ, RTO 4 giờ.
- Log JSON có trace\_id; metrics Prometheus; trace OpenTelemetry; cảnh báo khi hàng đợi tồn quá 10 phút, provider lỗi hoặc chi phí vượt 80% ngân sách.
- Migration DB có phiên bản, tương thích ngược; nâng cấp gây gián đoạn ngắn (vài giây mỗi container).

## 8. Kiến trúc và tech stack

Go xử lý mọi request người dùng, agent và mọi lời gọi tới LLM provider (model gateway); Python chỉ xử lý tài liệu. Hai bên giao việc qua Redis Streams; worker gọi model gateway nội bộ của Go khi cần embed.

&#91;embedded content: kiến trúc · 4 khối xử lý, 3 kho lưu trữ\]

Go là cửa duy nhất kiểm quyền và nơi duy nhất giữ API key của provider; worker Python không nhận request từ người dùng, chỉ nhận job từ hàng đợi.

Kiến trúc đầy đủ (sơ đồ, tech stack, cấu trúc code, vòng đời dữ liệu, Docker): Kiến trúc. ADR, API contract, hàng đợi, schema, bảo mật: Thiết kế chi tiết.

### 8.1 Tech stack

| Tầng | Công nghệ | Ghi chú |
| --- | --- | --- |
| Frontend | Next.js (App Router), TypeScript | SSE cho chat và trạng thái; PDF viewer để tô sáng trích dẫn |
| Backend | Go | REST + SSE; agent; model gateway (SDK của OpenAI, Anthropic, Gemini); Qdrant gRPC client; OpenTelemetry |
| Xử lý tài liệu | Python | Parse PDF/DOCX/XLSX/PPTX, crawl URL (headless browser khi cần), chunk, gọi embed |
| Hàng đợi, cache | Redis | Streams + consumer group + DLQ; Pub/Sub trạng thái; cache quyền, rate limit, semantic cache |
| CSDL | Postgres | Dữ liệu nghiệp vụ, trace, eval, audit |
| Vector DB | Qdrant | Dense + sparse trong cùng point; payload index kb\_id (tenant) |
| File | MinIO | File gốc, text đã parse (JSON có vị trí), bật SSE + versioning |
| LLM provider | OpenAI, Azure OpenAI, Anthropic, Gemini, Cohere, Voyage, Jina (qua API) | Gọi qua model gateway trong Go; khai báo trong Settings |
| Hạ tầng | Docker Compose, Nginx, Prometheus, Grafana | 2 máy chủ: ứng dụng, dữ liệu |

### 8.2 Luồng nạp tài liệu

1. Next.js xin presigned URL từ Go (kiểm quyền Contributor trở lên, kiểm quota), upload thẳng lên MinIO.
2. Next.js báo hoàn tất; Go tạo bản ghi `documents` (Queued) và đẩy job vào stream `ingest.parse`.
3. Worker parse: kiểm tra file, parse, lưu text có vị trí vào MinIO, đẩy sang `ingest.chunk`.
4. Worker chunk: chia chunk theo cấu hình KB, ghi `chunks` vào Postgres, đẩy sang `ingest.embed`.
5. Worker embed: gọi model gateway của Go để embed theo batch, tính sparse BM25, upsert Qdrant với point ID = chunk\_id, đánh dấu Indexed.
6. Mỗi bước ghi `pipeline_events` và publish Redis; Go đẩy SSE tới UI.

URL đi qua bước `ingest.crawl` trước bước 3; mỗi trang crawl được thên thành 1 tài liệu con.

### 8.3 Luồng hỏi đáp

1. Go xác thực JWT, lấy danh sách KB user có quyền (cache Redis).
2. Agent loop gọi model `chat` với tool; khi gọi `search_kb`, Go embed câu hỏi, truy vấn Qdrant (filter kb\_id), rerank.
3. Lặp tối đa 5 lần gọi tool, sau đó sinh câu trả lời kèm trích dẫn, stream SSE.
4. Ghi message, citation và trace vào Postgres (bất đồng bộ).

### 8.4 Tổ chức Qdrant

- Mỗi embedding model 1 collection (ví dụ `emb_openai_te3_large_v1`), named vectors `dense` + `sparse`.
- `kb_id` là payload index loại tenant; thêm index cho `doc_id`, `lang`, `classification`.
- Re-index 1 KB: ghi point với generation mới, xong thì đổi active\_generation của KB (truy vấn luôn lọc theo generation); point cũ giữ 7 ngày để rollback. Đổi embedding model thì ghi sang collection của model mới.

### 8.5 Data model chính (Postgres)

| Bảng | Trường chính |
| --- | --- |
| users | id, email, password\_hash, display\_name, avatar\_url, locale, theme, google\_sub (P1), is\_system\_admin, status |
| sessions | id, user\_id, refresh\_token\_hash, family\_id, expires\_at, revoked\_at |
| workspaces | id, name, owner\_id |
| knowledge\_bases | id, workspace\_id, name, description, chunk\_config, embedding\_model\_id, chat\_model\_id, rerank\_model\_id, prompt\_config, active\_generation, qdrant\_collection |
| memberships | subject\_type (user/group), subject\_id, scope\_type (workspace/kb), scope\_id, role |
| documents | id, kb\_id, source\_type (file/url), source\_uri, sha256, status, error\_code, classification, version |
| chunks | id, document\_id, kb\_id, ordinal, text\_hash, token\_count, page, heading\_path, lang |
| pipeline\_events | id, document\_id, stage, status, started\_at, finished\_at, worker\_id, detail (JSON) |
| model\_providers, models | provider: type, base\_url, encrypted\_key, data\_policy; model: role, dims, context\_window, price\_in, price\_out, rpm, tpm, is\_default |
| model\_usage | model\_id, workspace\_id, kb\_id, user\_id, purpose (chat/embed/rerank/eval), tokens\_in, tokens\_out, cost, at |
| quotas | scope (workspace/user), storage\_gb, max\_documents, tokens\_per\_day, monthly\_budget |
| conversations, messages, citations | message: role, content, trace\_id; citation: message\_id, chunk\_id, score |
| traces, trace\_spans | span: type, input, output, latency\_ms, tokens\_in, tokens\_out, cost |
| eval\_datasets, eval\_items, eval\_runs, eval\_results | run: config\_snapshot, metrics (JSON) |
| feedback, audit\_logs | audit: actor\_id, action, target, before, after, ip, at |

## 9. Chỉ số thành công, giả định và rủi ro

### 9.1 Chỉ số thành công (3 tháng sau go-live)

| Nhóm | Chỉ số | Mục tiêu |
| --- | --- | --- |
| Chất lượng truy xuất | Hit@5 trên Eval dataset | ≥ 85% |
| Chất lượng câu trả lời | Faithfulness | ≥ 90% |
| Chất lượng câu trả lời | Độ chính xác trích dẫn | ≥ 95% |
| Chất lượng câu trả lời | Tỉ lệ từ chối đúng (câu ngoài tài liệu) | ≥ 90% |
| Hài lòng | Tỉ lệ like / (like + dislike) | ≥ 80% |
| Sử dụng | User hoạt động hằng tuần / tổng user được cấp | ≥ 60% |
| Pipeline | Tài liệu Indexed thành công ngay lần đầu (loại trừ Needs OCR) | ≥ 98% |
| Hiệu năng | Token đầu tiên p95 | dưới 3 giây |
| Bảo mật | Sự cố lộ dữ liệu chéo KB | 0 |

### 9.2 Giả định

- Quy mô: 500 user, 100 đồng thời, 100.000 tài liệu sau 1 năm.
- Công ty có hợp đồng doanh nghiệp với ít nhất 1 LLM provider (không dùng dữ liệu để huấn luyện), và cho phép gửi nội dung tài liệu nội bộ tới provider đó.
- Máy chủ ứng dụng được ra Internet tới domain của provider qua egress proxy.
- Model embedding và rerank của provider hỗ trợ đa ngôn ngữ (vi, en, zh); model chat hỗ trợ tool calling. Model cụ thể chọn qua Eval.
- Đăng nhập bằng email + mật khẩu ở Phase 1; Google ở Phase 2; chưa cần AD/LDAP.
- URL Phase 1 là trang truy cập được không cần đăng nhập.
- Lộ trình chia theo phạm vi, chưa gắn ngày.

### 9.3 Rủi ro

| Rủi ro | Ảnh hưởng | Giảm thiểu |
| --- | --- | --- |
| Nội dung nội bộ rời công ty khi gọi provider | Rủi ro bảo mật, tuân thủ (Nghị định 13/2023 về dữ liệu cá nhân) | Hợp đồng doanh nghiệp + zero data retention nếu có; egress proxy chỉ mở domain provider; che PII (P1); provider riêng cho KB Confidential (P1) |
| Provider lỗi, bị rate limit hoặc ngừng một model | Chat chậm hoặc lỗi, hàng đợi embed tồn | Retry có backoff, ưu tiên chat hơn embed nền; fallback model (P1); không phụ thuộc một provider |
| Chi phí model vượt dự kiến (đặc biệt embed hàng loạt, re-index, Eval) | Vượt ngân sách | Hiện ước tính chi phí trước khi re-index; ngân sách tháng + cảnh báo 80%; semantic cache; chọn model utility rẻ cho việc phụ |
| Đổi embedding model bắt buộc re-index toàn bộ KB | Tốn thời gian và chi phí | Chọn kỹ qua Eval từ đầu; re-index chạy nền theo generation, không gián đoạn |
| Parse PDF/XLSX phức tạp sai cấu trúc | Chunk rác, truy xuất kém | Inspector + sửa chunk; bộ tài liệu mẫu khó trong test |
| Tự viết agent loop và model gateway bằng Go | Tốn công hơn dùng framework Python | Dùng SDK chính thức của OpenAI, Anthropic, Gemini; giữ vòng lặp đơn giản (ReAct + 3 tool) |
| Người dùng không tin câu trả lời | Ít dùng | Trích dẫn bấm được, tiến trình agent hiển thị, từ chối khi không chắc |
| Tài liệu scan nhiều hơn dự kiến | Phase 1 thiếu dữ liệu | Đếm số file `Needs OCR` ngay từ Phase 1 để quyết định kéo OCR sớm |

### 9.4 Câu hỏi mở

- [ ] Provider nào được phép dùng (pháp chế/IT duyệt) và loại dữ liệu nào được gửi tới provider?
- [ ] Ngân sách chi phí model mỗi tháng?
- [ ] Xác nhận con số tải thật (user, đồng thời, tài liệu).
- [ ] Chọn model chat, embedding, rerank cụ thể (chạy Eval so sánh 2–3 ứng viên mỗi loại).
- [ ] Đăng ký tự do theo domain email công ty hay chỉ qua lời mời của Admin?
- [ ] Chính sách lưu hội thoại và trace: bao lâu, ai được xem?
- [ ] Quy mô team và deadline để gắn ngày cho từng phase.
