---
name: address-comments
description: Xử lý comment review của Owner trên một PR - gom comment, giao lại cho agent viết code sửa trên đúng branch, chạy cổng chất lượng, push, trả lời từng comment, chạy lại review tự động. Owner gọi bằng /address-comments <số-PR>.
disable-model-invocation: true
argument-hint: <số-PR>
---

# /address-comments $ARGUMENTS

1. **Lấy comment:**
   ```bash
   gh pr view $ARGUMENTS --json number,headRefName,title,body,reviews,comments
   gh api repos/{owner}/{repo}/pulls/$ARGUMENTS/comments --paginate
   ```
   Chỉ lấy comment chưa resolve của Owner hoặc reviewer là người. Tìm mã task trong tiêu đề PR, rồi mở file task.
2. **Phân loại** từng comment:
   - `fix`: yêu cầu sửa rõ ràng;
   - `question`: Owner hỏi, cần trả lời;
   - `discuss`: không đồng ý hoặc cần quyết định.

   Với `discuss`: **không tự sửa**. Đề xuất phương án cho Owner trong báo cáo.
3. **Giao việc:** spawn `owner` của task, đặt `name` là mã task. Lời giao việc gồm:
   - đường dẫn task;
   - lệnh checkout branch đã có: `git fetch origin && git switch <headRefName>`;
   - danh sách comment `fix`, mỗi comment có `file:line`, nguyên văn và id;
   - yêu cầu: mỗi comment một commit `fix(<area>): address review — <tóm tắt> [<TASK-ID>]`, không làm thêm ngoài comment, báo cáo theo `task-contract` kèm bảng `comment-id → commit`.
4. **Kiểm:** cổng chất lượng như Bước 3 của `/run-wave`, rồi `git push`.
5. **Trả lời từng comment** bằng `gh api .../pulls/$ARGUMENTS/comments/<id>/replies -f body="Đã sửa ở <sha>: …"`. Comment loại `question` thì trả lời bằng nội dung mà agent hoặc bạn đã kiểm chứng.
6. Chạy lại review tự động (Bước 5 của `/run-wave`) trên phần thay đổi mới. Sạch thì re-request review: `gh pr edit $ARGUMENTS --add-reviewer <owner-login>`.
7. Báo Owner: số comment đã xử lý, các commit, các mục `discuss` chờ Owner quyết định.
