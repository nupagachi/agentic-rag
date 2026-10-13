---
name: security-standards
description: Chuẩn bảo mật ứng dụng cho hệ thống RAG on-prem dùng LLM provider ngoài - OWASP ASVS 4.0 L2, OWASP Top 10, API Security Top 10, OWASP Top 10 for LLM Applications; danh sách mối đe dọa riêng của dự án, công cụ quét, thang mức độ và định dạng kết quả. Dùng khi threat model hoặc review bảo mật.
user-invocable: false
---

# Chuẩn bảo mật

Khung tham chiếu: **OWASP ASVS 4.0 mức L2**, OWASP Top 10 (2021), OWASP API Security Top 10 (2023), **OWASP Top 10 for LLM Applications (2025)**, CWE Top 25.

## 1. Mối đe dọa riêng của dự án (kiểm theo thứ tự)
| # | Mối đe dọa | Phải kiểm | Tham chiếu |
| --- | --- | --- | --- |
| T1 | **Rò dữ liệu giữa KB, IDOR** | Mọi truy vấn theo KB đi qua `CanAccessKB` / `AllowedKBIDs`; filter Qdrant do server dựng; ID trong URL được kiểm quan hệ thuộc về; trace, Inspector, Eval cũng lọc theo quyền | API1, API5, ASVS V4 |
| T2 | **Prompt injection gián tiếp** qua tài liệu và trang crawl | Nội dung truy xuất được chèn vào vùng phân định, coi là dữ liệu; tool của agent có allowlist, chỉ đọc, tham số được validate; không có tool có tác dụng phụ được kích hoạt bởi nội dung tài liệu; không lộ system prompt | LLM01 |
| T3 | **Đưa dữ liệu nhạy cảm ra provider** | Chỉ gửi đúng phần cần (câu hỏi, chunk, lịch sử đã cắt); không gửi secret, PII thừa hay metadata nội bộ; mọi lời gọi đi qua gateway và squid :3128 | LLM02, LLM07 |
| T4 | **SSRF qua crawl URL** | Chỉ qua squid :3129; chặn dải IP nội bộ, link-local, metadata; chỉ scheme http/https; kiểm lại sau mỗi redirect; chống DNS rebinding (proxy tự resolve) | API7, CWE-918 |
| T5 | **Upload file độc hại** | Giới hạn kích thước; MIME sniff khớp allowlist; quét ClamAV; giới hạn giải nén, số trang, thời gian parse; key object MinIO do server sinh; presigned URL TTL ≤ 15 phút, có `content-length-range` | CWE-434, CWE-409 |
| T6 | **Xác thực, phiên** | argon2id (tối thiểu m=19 MiB, t=2, p=1); JWT RS256 với `alg` cố định, `exp` ≤ 15 phút, kiểm `iss`/`aud`; refresh xoay vòng và phát hiện dùng lại; cookie `HttpOnly; Secure; SameSite=Strict; Path=/api/auth`; rate limit và khóa tạm | ASVS V2, V3 |
| T7 | **Secret** | API key provider mã hóa AES-256-GCM bằng master key, chỉ trả về dạng che (`sk-…abcd`), không bao giờ log; `internal_token` so sánh constant-time; `/internal/*` không ra Nginx | ASVS V6, V14 |
| T8 | **Injection** | Chỉ dùng sqlc hoặc query có tham số; không `exec` lệnh shell bằng input; template prompt không cho chèn chỉ dẫn hệ thống từ input | A03 |
| T9 | **XSS** từ nội dung LLM hoặc tài liệu | Markdown được sanitize, không raw HTML; có header CSP; tên file và tiêu đề được escape; preview PDF dùng PDF.js, không nhúng HTML gốc | A03, LLM05 |
| T10 | **CSRF** | Endpoint dùng cookie (refresh, logout) kiểm `Origin` và cùng SameSite; API khác dùng Bearer | ASVS V13 |
| T11 | **Lạm dụng tài nguyên, chi phí** | Rate limit theo user/IP; quota token theo workspace; `max_tokens`; giới hạn số vòng agent; timeout; giới hạn kích thước request | LLM10, API4 |
| T12 | **Audit** | Đăng nhập, đổi quyền, thêm hoặc sửa provider và key, xóa KB/tài liệu, xuất dữ liệu đều ghi `audit_logs` (ai, gì, khi nào, IP, kết quả) | ASVS V7 |
| T13 | **Chuỗi cung ứng** | Dependency có lockfile, pin phiên bản; dependency mới phải có lý do; image dùng distroless/nonroot; có SBOM | A06, A08 |
| T14 | **Log, lỗi** | Không log token, key, mật khẩu, nội dung tài liệu ở mức info; lỗi trả client không lộ stack hay SQL | A09, ASVS V7 |

## 2. Công cụ (`make sec`)
- `gitleaks detect --no-banner`: tìm secret.
- `semgrep ci --config p/owasp-top-ten --config p/golang --config p/python --config p/typescript --config p/secrets`: SAST.
- `govulncheck ./...` (api), `uv run pip-audit` (workers), `npm audit --audit-level=high` (web).
- `trivy fs --severity HIGH,CRITICAL --exit-code 1 .`. CI thêm `trivy image` cho image đã build.

Kết quả của công cụ chỉ là gợi ý. Chỉ đưa vào báo cáo những gì liên quan tới diff và đã xác minh.

## 3. Mức độ
| Mức | Ý nghĩa | Xử lý |
| --- | --- | --- |
| Critical | Khai thác từ xa không cần quyền, hoặc rò dữ liệu giữa KB/workspace, lộ secret | Chặn merge, sửa ngay |
| High | Khai thác được khi có tài khoản thường; injection; leo quyền | Chặn merge |
| Medium | Cần điều kiện đặc biệt; thiếu một lớp phòng thủ | Sửa trong PR, hoặc Owner chấp nhận và tạo ticket |
| Low / Info | Hardening, best practice | Ghi chú |

## 4. Định dạng kết quả
### Threat model (`MODE: threat-model`)
```markdown
## Threat model <SPEC-ID>
### Luồng dữ liệu bị ảnh hưởng
- <nguồn> → <xử lý> → <đích> (ranh giới tin cậy: …)
### STRIDE
| Thành phần | S | T | R | I | D | E | Ghi chú |
### Yêu cầu bảo mật
| SR | Yêu cầu (kiểm chứng được) | Áp cho task | Test gợi ý | Tham chiếu |
| SR-1 | Endpoint X trả 404 khi kbId thuộc workspace khác | M2-T03 | TestGetKB_OtherWorkspace | T1, API1 |
```

### Review PR (`MODE: pr-review`)
```markdown
## Security review PR #<n> (<TASK-ID>)
| Mức | Vị trí | Vấn đề | Kịch bản khai thác | Cách sửa | Tham chiếu |
| High | api/internal/kb/handler.go:88 | … | … | … | T1 / ASVS 4.2.1 |
### Công cụ
- gitleaks: 0 · semgrep: 1 (đã xác minh là false positive vì …) · govulncheck: 0
VERDICT: PASS | FAIL
```
