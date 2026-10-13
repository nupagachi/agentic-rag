---
name: code-review-standards
description: Checklist và định dạng review code của review-agent - đúng AC, đúng phạm vi, kiến trúc, lỗi, đồng thời, hiệu năng, test, quan sát, migration, i18n; thang mức độ blocker/major/minor/nit và verdict. Dùng khi review một PR.
user-invocable: false
---

# Chuẩn review code

## 1. Checklist (theo thứ tự ưu tiên)
1. **Đúng yêu cầu**:
   - mỗi AC và SR trong task được đáp ứng;
   - logic đúng ở trường hợp biên (rỗng, null, trùng, đồng thời, quyền).
2. **Đúng phạm vi**:
   - chỉ sửa trong `scope.paths`;
   - không làm thêm tính năng ngoài task;
   - không tự refactor lan sang chỗ khác.
3. **Kiến trúc**:
   - đúng layering (handler, service, repo; stage và runtime);
   - dùng code sinh từ contracts;
   - LLM chỉ gọi qua gateway;
   - không phá ranh giới giữa các module.
4. **Lỗi và độ bền**:
   - lỗi được xử lý và wrap đúng;
   - có timeout, retry đúng chỗ;
   - idempotent;
   - transaction không bao lời gọi mạng.
5. **Đồng thời**: race, goroutine rò rỉ, deadlock, thiếu hủy theo context; state React bị dùng sai.
6. **Hiệu năng**:
   - truy vấn N+1;
   - query không giới hạn hoặc thiếu phân trang;
   - thiếu index cho query mới;
   - nạp cả file vào RAM;
   - re-render thừa; bundle phình.
7. **Test**:
   - test chứng minh được AC;
   - assert hành vi, không assert cách cài đặt;
   - có ca lỗi và ca không có quyền;
   - xác định (không sleep, không gọi mạng thật).
8. **Quan sát**: log có ngữ cảnh và không chứa dữ liệu nhạy cảm; có span và metric cho luồng mới.
9. **Dữ liệu**:
   - migration có cả down;
   - an toàn khi chạy song song với bản cũ;
   - không khóa bảng lớn.
10. **UI** (nếu có): theo `fe-ui-design` (token, đủ 4 trạng thái, i18n, a11y, layout một màn hình).
11. **Dễ đọc**: tên rõ nghĩa; hàm ngắn; comment giải thích "vì sao", không lặp lại "cái gì".

## 2. Mức độ
| Mức | Định nghĩa | Hệ quả |
| --- | --- | --- |
| `blocker` | Sai AC, bug chắc chắn, mất hoặc rò dữ liệu, vi phạm kiến trúc cốt lõi | REQUEST_CHANGES; agent viết code sửa |
| `major` | Bug có điều kiện, thiếu test cho AC, vấn đề hiệu năng rõ, khó bảo trì đáng kể | REQUEST_CHANGES |
| `minor` | Cải thiện nên làm nhưng không chặn | Ghi lại; Owner quyết định |
| `nit` | Sở thích, cách gọi tên | Tối đa 3 mục, có thể bỏ |

## 3. Định dạng kết quả
```markdown
## Code review PR #<n> (<TASK-ID>)
Tóm tắt: <1–2 câu: PR làm gì, chất lượng chung>

| # | Mức | Vị trí | Vấn đề | Đề xuất sửa |
| 1 | blocker | api/internal/document/service.go:142 | Xóa point Qdrant trước khi commit Postgres; nếu commit fail thì mất vector | Commit Postgres trước, sau đó gửi job xóa vector |

### AC → bằng chứng (đã tự kiểm)
| AC | Trạng thái | Ghi chú |
| AC1 | ✅ | TestLogin_ValidCredentials |

### Owner nên xem kỹ
- <những điểm cần con người quyết định: đánh đổi thiết kế, UX, nghiệp vụ>

VERDICT: APPROVE | REQUEST_CHANGES
```
Mọi phát hiện phải dựa trên code đã đọc thật. Nghi ngờ mà chưa kiểm chứng được thì ghi vào "Owner nên xem kỹ", không xếp vào blocker.
