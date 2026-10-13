---
name: plan-spec
description: Biến một SPEC thành plan gồm các task nhỏ FE/BE/test theo wave, kèm threat model. Owner gọi bằng /plan-spec <đường-dẫn-spec>.
disable-model-invocation: true
argument-hint: <docs/specs/SPEC-ID.md>
---

# /plan-spec $ARGUMENTS

Bạn là Orchestrator. Lập plan cho SPEC `$ARGUMENTS`. **Không viết code, không spawn agent viết code.**

## Bước 1: Kiểm SPEC đủ điều kiện (Definition of Ready)
Đọc SPEC, các mục PRD mà nó tham chiếu, `docs/Kiến trúc.md` và module tương ứng trong `docs/Phân rã P0.md`. SPEC phải có:
- mục tiêu và ngoài phạm vi;
- user story;
- AC dạng Given/When/Then;
- thay đổi API hoặc dữ liệu (hoặc ghi "không có");
- màn hình UI liên quan;
- yêu cầu phi chức năng.

Thiếu hoặc mâu thuẫn thì liệt kê câu hỏi đánh số rồi **dừng lại**.

## Bước 2: Thu thập song song (spawn trong cùng một message)
- `security-agent` với `MODE: threat-model`, truyền đường dẫn SPEC. Kết quả mong đợi là bảng STRIDE và danh sách SR.
- `Explore` (độ kỹ "medium"): lập bản đồ code hiện có bị ảnh hưởng (domain, bảng, component, route) và các mẫu code nên bắt chước.

## Bước 3: Chia task theo các luật sau
1. **1 task = 1 PR = 1 owner = 1 area.** Kích thước S (≤150 dòng) hoặc M (≤400 dòng). Lớn hơn thì tách tiếp.
2. **Wave 0 là contracts.** Gom mọi thay đổi `contracts/` vào 1–2 task do `be-agent` làm. Nếu SPEC có migration nền thì cũng đặt ở wave 0.
3. **Chia theo lát dọc trong từng tầng**, mỗi task ra kết quả kiểm được. Ví dụ "API đăng nhập + test", không chia kiểu "viết hết repo trước".
4. **FE và BE song song từ wave 1.** FE dựa trên client sinh từ contracts và mock bằng MSW, không cần chờ BE xong.
5. **Không có hai task cùng wave sửa chung file.** Liệt kê `scope.paths` cụ thể để kiểm.
6. **Mỗi task có AC kiểm được.** Gắn SR từ threat model vào task tương ứng.
7. **Task của test-agent:**
   - một task test tích hợp/contract cho phần BE;
   - một task e2e cho các luồng UI chính;
   - một task test RAG khi SPEC đụng retrieval, agent hoặc chunk.

   Task test được chạy song song với task code; `depends_on` dùng để biết khi nào được đánh dấu ready.
8. Đặt mã task là `<MODULE>-T<nn>` (ví dụ `M1-T03`) và ghi mã task P0 tương ứng, nếu có.

## Bước 4: Ghi file
- `docs/plans/<SPEC-ID>/plan.md` theo mẫu [plan-template.md](plan-template.md).
- `docs/plans/<SPEC-ID>/tasks/<TASK-ID>.md` theo mẫu [task-template.md](task-template.md). Trạng thái ban đầu: wave 0 là `ready`, còn lại là `todo`.
- `docs/plans/<SPEC-ID>/threat-model.md`: kết quả nguyên văn của security-agent.

## Bước 5: Trình Owner duyệt (điểm dừng 1)
Trả lời Owner gồm:
- bảng wave, mỗi dòng ghi: task, owner, size, depends_on;
- tổng số task, số PR dự kiến;
- rủi ro chính;
- câu hỏi cần quyết định.

Kết thúc bằng: "Duyệt plan thì chạy `/run-wave <SPEC-ID> 0`." **Không tự chạy wave.**
