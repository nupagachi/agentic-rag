---
name: task-status
description: Báo trạng thái các SPEC/task/PR đang chạy, đồng bộ với GitHub. Owner gọi bằng /task-status [SPEC-ID].
disable-model-invocation: true
argument-hint: "[SPEC-ID]"
allowed-tools: Read Grep Glob Bash(gh pr view *) Bash(gh pr list *) Bash(git worktree list*)
---

# /task-status $ARGUMENTS

## Dữ liệu hiện tại
- PR do agent mở: !`gh pr list --label agent --state all --limit 50 --json number,title,state,isDraft,labels,reviewDecision`
- Worktree: !`git worktree list`

## Việc cần làm
1. Đọc `docs/plans/$ARGUMENTS/plan.md`. Không có tham số thì đọc mọi `docs/plans/*/plan.md`.
2. Đối chiếu với danh sách PR ở trên, chỉ ra chỗ lệch (plan ghi `in-review` nhưng PR đã merge, v.v.). Chỉ cập nhật file plan khi Owner đồng ý.
3. Trả bảng theo SPEC: `wave | task | owner | status | PR | ghi chú`. Sau bảng ghi:
   - task blocked;
   - PR chờ Owner;
   - wave tiếp theo có thể chạy chưa.
