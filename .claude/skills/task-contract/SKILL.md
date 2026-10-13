---
name: task-contract
description: Hợp đồng làm việc chung cho mọi agent viết code - đọc file task, giữ đúng phạm vi, quy ước git/commit, Definition of Done, định dạng báo cáo trả về Orchestrator. Dùng cho mọi task trong docs/plans/.
user-invocable: false
---

# Task contract

## 1. File task là hợp đồng
Mỗi task nằm ở `docs/plans/<SPEC-ID>/tasks/<TASK-ID>.md`. Phần frontmatter gồm:
- `id`, `owner`, `area`, `wave`, `depends_on`, `branch`;
- `scope.paths` (được sửa), `scope.forbidden` (cấm sửa);
- `size`, `status`.

Phần thân gồm Mục tiêu, AC (Given/When/Then), SR (yêu cầu bảo mật), Yêu cầu test, Ngoài phạm vi.

- Chỉ sửa file khớp `scope.paths`. Ngoại lệ: file test cạnh code, file i18n, và code sinh ra bởi `make gen`.
- Cần sửa ngoài phạm vi, cần thêm dependency, hoặc AC mơ hồ hay mâu thuẫn: **dừng lại** và trả `status: blocked` kèm `SCOPE_CHANGE_NEEDED` hoặc `QUESTION`, mô tả cụ thể. Không tự đoán ý nghiệp vụ.
- Kích thước diff:
  - `S` ≤ 150 dòng;
  - `M` ≤ 400 dòng (không tính code sinh ra, lockfile, snapshot).

  Vượt quá thì đề xuất cách tách task.

## 2. Git
- Branch: đúng giá trị `branch` trong task, theo mẫu `<type>/<TASK-ID>-<slug>`, ví dụ `feat/M1-T03-login-api`.
- Commit theo Conventional Commits, viết tiếng Anh, có mã task:
  ```
  feat(api): add login endpoint with RS256 access token [M1-T03]
  test(api): cover refresh token reuse detection [M1-T04]
  ```
  `type` ∈ feat, fix, test, refactor, perf, docs, chore, build, ci. `scope` ∈ web, api, workers, contracts, deploy, gateway, agent.
- Commit nhỏ, mỗi commit build được. Không dùng `--no-verify`, không `git push`, không rebase hay sửa lịch sử của `main`.
- Không commit:
  - secret;
  - `.env`;
  - file trong `deploy/secrets/`;
  - file lớn hơn 1MB (trừ `testdata/` đã được duyệt);
  - code debug, `console.log`, `fmt.Println`, `print`.

## 3. Definition of Done
- [ ] Mỗi AC và SR có code đáp ứng và có test chứng minh.
- [ ] `make check-<area>` xanh trên máy (worktree).
- [ ] Không có TODO thiếu mã task (`TODO(M1-T07): ...` thì được).
- [ ] Có i18n vi/en cho chuỗi mới; có migration up và down nếu đổi schema.
- [ ] Log, trace, metric cho luồng mới theo convention.
- [ ] Không thêm dependency ngoài danh sách được cho phép trong task.
- [ ] Tài liệu: cập nhật README của module hoặc ADR nếu đổi quyết định kiến trúc.

## 4. Báo cáo trả về (bắt buộc đúng định dạng)
```markdown
## Kết quả <TASK-ID>
status: done | partial | blocked
branch: <branch>
worktree: <đường dẫn worktree>
commits: <số commit> — <sha ngắn> <subject> …
diff: +<thêm> / -<xóa> dòng (không tính code sinh ra)

### AC / SR → bằng chứng
| Mục | Đáp ứng ở | Test |
| --- | --- | --- |
| AC1 | api/internal/auth/service.go:Login | TestLogin_ValidCredentials |

### Kiểm tra đã chạy
- `make check-api` → PASS (… tests)

### Rủi ro, giả định, việc còn lại
- …

### Câu hỏi cho Owner (nếu có)
- …
```
Báo cáo phải trung thực. Test fail, bỏ qua bước, hay chưa chạy thì phải ghi rõ. Không viết "đã xong" khi chưa chạy kiểm tra.
