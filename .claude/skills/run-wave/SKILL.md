---
name: run-wave
description: Chạy một wave của plan - giao task song song cho fe/be/test-agent trong worktree riêng, chạy cổng chất lượng, mở PR draft, review tự động (review-agent + security-agent), vòng sửa, rồi chuyển PR cho Owner. Owner gọi bằng /run-wave <SPEC-ID> [wave].
disable-model-invocation: true
argument-hint: <SPEC-ID> [wave]
---

# /run-wave $ARGUMENTS

## Bước 0: Đồng bộ trạng thái
1. `git fetch origin --prune`. Với mỗi task có `pr`, chạy `gh pr view <n> --json state,mergedAt,reviewDecision` và cập nhật `status`:
   - đã merge → `merged`;
   - Owner request changes → `changes-requested`. Báo Owner dùng `/address-comments <n>`.
2. Dọn worktree của task đã merge: `git worktree list`, rồi `git worktree remove <path>` cho các worktree đó.
3. Plan chưa được Owner duyệt (ô "Duyệt bởi Owner" còn trống) thì dừng lại và nhắc Owner.

## Bước 1: Chọn task chạy được
Lấy wave trong tham số; nếu không có thì lấy wave nhỏ nhất còn task chưa xong. Một task được chạy khi:
- `status` là `todo` hoặc `ready`;
- mọi `depends_on` đã `merged`.

Ngoại lệ cho task `test-agent`: chỉ cần các task contract đã merge, các phụ thuộc còn lại chỉ cần `in-progress` trở lên.

Thêm các giới hạn:
- `scope.paths` của các task được chọn không được giao nhau;
- tối đa 4 task.

Ghi lại task bị loại và lý do.

## Bước 2: Giao việc song song
Gọi **tất cả** agent trong **cùng một message**, mỗi task một lời gọi Agent (`subagent_type` = `owner` của task; agent đã cấu hình `isolation: worktree`). Đặt `name` là mã task, ví dụ `M1-T03`, để có thể nhắn lại sau. Đặt status `in-progress`.

Mẫu lời giao việc (phải tự đủ nghĩa):
```
TASK: docs/plans/<SPEC-ID>/tasks/<TASK-ID>.md (đọc toàn bộ file trước khi làm)
SPEC: docs/specs/<SPEC-ID>.md
BRANCH: <branch> — tạo từ origin/main: git fetch origin && git switch -c <branch> origin/main
        (nếu branch đã có trên remote: git switch <branch>)
AC & SR: <dán nguyên văn>
SCOPE: <scope.paths>  · CẤM: <scope.forbidden>
KIỂM TRA: make check-<area> phải xanh trước khi báo done
KHÔNG push, KHÔNG mở PR. Commit xong trả báo cáo đúng định dạng skill task-contract.
```

## Bước 3: Cổng chất lượng (với từng agent khi nó báo xong)
1. Báo cáo phải đúng định dạng. Trường hợp `blocked` hoặc `SCOPE_CHANGE_NEEDED`: đặt status `blocked`, gom lại để báo Owner, không tự quyết.
2. Trong worktree của task, chạy `make check-<area>`. Nếu đụng `contracts/` thì chạy thêm `make gen && git diff --exit-code`.
3. Fail thì gửi log lỗi (đã rút gọn) lại cho agent bằng SendMessage tới `<TASK-ID>`, tối đa 1 lần. Vẫn fail thì `blocked`.
4. Kiểm diff: `git diff --stat origin/main...<branch>`. Có file ngoài `scope.paths` thì trả lại agent.

## Bước 4: Mở PR draft
```bash
git push -u origin <branch>
gh pr create --draft --base main --head <branch> \
  --title "<type>(<area>): <title> [<TASK-ID>]" \
  --body-file <file tạm sinh từ .github/pull_request_template.md, đã điền task, AC→bằng chứng, kiểm tra, rủi ro> \
  --label agent --label "area/<area>" --label "wave/<n>"
```
Ghi số PR vào file task và vào `plan.md`.

## Bước 5: Review tự động (chạy song song cho mọi PR của wave)
- Trong **cùng một message**, mỗi PR spawn:
  - `review-agent`: số PR, branch, đường dẫn task;
  - `security-agent`: `MODE: pr-review`, số PR, branch, đường dẫn task, đường dẫn threat model.
- Gộp hai kết quả thành một comment rồi đăng: `gh pr review <n> --comment --body-file <tmp>`.
- Nếu có `blocker`/`major` hoặc `Critical`/`High`:
  1. đặt status `changes-requested`;
  2. gửi danh sách phát hiện cho agent viết code (SendMessage `<TASK-ID>`; nếu agent không còn thì spawn mới với lời giao việc của Bước 2 cộng danh sách phát hiện; branch đã có);
  3. agent sửa xong thì làm lại Bước 3 → push → Bước 5.

  Tối đa **2 vòng**. Hết 2 vòng vẫn còn thì gắn nhãn `needs-human` và báo Owner.
- `test-agent` có PR mà phụ thuộc chưa merge thì giữ draft, gắn nhãn `blocked-by-deps`.

## Bước 6: Chuyển cho Owner
PR đã sạch:
```bash
gh pr ready <n>
gh pr edit <n> --add-label ready-for-human
```
Đặt status `in-review`. Ghi nhật ký vào `plan.md`.

## Bước 7: Báo cáo (điểm dừng 2)
Bảng: `task | agent | PR | CI | review | security | vòng sửa | Owner cần làm`. Sau bảng là các task blocked kèm câu hỏi. Kết thúc bằng:

"Review và merge các PR `ready-for-human`. Có comment thì chạy `/address-comments <n>`. Merge xong thì chạy `/run-wave <SPEC-ID>` để sang wave tiếp."
