---
name: review-pr
description: Chạy review tự động (review-agent + security-agent song song) cho một PR bất kỳ, kể cả PR do người viết, rồi đăng kết quả lên PR. Owner gọi bằng /review-pr <số-PR>.
disable-model-invocation: true
argument-hint: <số-PR>
---

# /review-pr $ARGUMENTS

1. `gh pr view $ARGUMENTS --json number,title,headRefName,baseRefName,body,files`. Tìm mã task trong tiêu đề, nếu có.
2. Trong **cùng một message**, spawn:
   - `review-agent`: số PR, branch, đường dẫn task (hoặc "không có task: review theo mô tả PR");
   - `security-agent`: `MODE: pr-review`, số PR, branch.
3. Gộp kết quả, đăng bằng `gh pr review $ARGUMENTS --comment --body-file <tmp>`. Không dùng `--approve` và không merge; quyết định thuộc về Owner.
4. Tóm tắt cho Owner: verdict của từng agent, số phát hiện theo mức độ, ba điểm Owner nên xem kỹ.
