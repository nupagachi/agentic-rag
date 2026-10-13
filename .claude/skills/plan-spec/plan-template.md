# Plan <SPEC-ID>: <tên SPEC>

SPEC: `docs/specs/<SPEC-ID>.md` · Threat model: `threat-model.md` · Lập: <ngày> · Duyệt bởi Owner: <ngày | chưa>

## Tóm tắt
<2–3 câu: SPEC làm gì, chia thành bao nhiêu task, mấy wave, rủi ro lớn nhất>

## Wave
| Wave | Task | Owner | Area | Size | Phụ thuộc | Trạng thái | PR |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 0 | M1-T01 Contract auth endpoints + errors | be-agent | contracts | S | — | ready | |
| 0 | M1-T02 Migration users, sessions | be-agent | api | S | — | ready | |
| 1 | M1-T03 API đăng nhập | be-agent | api | M | T01, T02 | todo | |
| 1 | M1-T09 UI đăng nhập/đăng ký | fe-agent | web | M | T01 | todo | |
| 1 | M1-T10 Test tích hợp + contract auth | test-agent | api | M | T01 (ready), T03 (merge) | todo | |
| 2 | … | | | | | | |

## Đồ thị phụ thuộc
```
T01 ─┬─> T03 ─> T04 ─> T07
     ├─> T09
     └─> T10 (chạy song song, ready khi T03 merge)
T02 ──> T03
```

## Rủi ro và cách giảm
| Rủi ro | Ảnh hưởng | Cách giảm |
| --- | --- | --- |

## Câu hỏi mở
1. …

## Nhật ký
- <ngày> Lập plan.
- <ngày> Wave 0: PR #12, #13 ready-for-human.
