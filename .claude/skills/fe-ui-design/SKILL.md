---
name: fe-ui-design
description: Design system và chuẩn thẩm mỹ giao diện RAG Studio (shadcn/ui + Lucide + Tailwind 4, token màu, typography Geist, spacing, motion, layout 1 màn hình). Dùng khi dựng hoặc sửa bất kỳ giao diện nào trong web/.
user-invocable: false
paths: web/**
---

# Chuẩn giao diện RAG Studio

Mục tiêu: giao diện sạch, nhiều khoảng trắng, chữ rõ ràng, chuyển động tinh tế, cảm giác "đắt tiền" như creativeforce.io hay Linear. Nguồn tham chiếu trực quan là UI canvas trong SPEC.

## 1. Token: không dùng màu hex trong component
Khai báo trong `web/app/globals.css` (Tailwind 4 `@theme`). Component chỉ dùng class token.

| Token | Light | Dùng cho |
| --- | --- | --- |
| `--foreground` / ink | `#0A0A14` | chữ chính, nút primary |
| `--brand` | `#435CFF` (hover `#2B41D9`) | điểm nhấn, link, focus ring, trạng thái active |
| `--brand-soft` | `#ECEFFF` | nền badge brand, hàng được chọn |
| `--border` | `#E5E6EB` | viền card, input, divider |
| `--muted` | `#F4F4F6` | nền tab list, skeleton |
| `--muted-foreground` | `#6B6E7B` | chữ phụ, label |
| `--sidebar` | `#FAFAFB` | nền sidebar |
| success / warning / destructive | theo shadcn | badge trạng thái, alert |

Bắt buộc có dark mode: định nghĩa lại token dưới `.dark`.

## 2. Typography
- Font: `Geist` cho chữ, `Geist Mono` cho số liệu, ID, token count, code. Nạp bằng `next/font`.
- Thang chữ:
  - 12/16 cho caption;
  - 13/20 cho bảng và chữ phụ;
  - 14/20 cho body mặc định;
  - 16/24;
  - 20/28 cho tiêu đề trang;
  - 24/32 và 32/40 chỉ dùng cho trang trống hoặc trang chào.
- Tiêu đề dùng `font-semibold` và `tracking-tight`. Không dùng quá 2 độ đậm trên một màn hình.
- Số liệu canh phải và dùng `tabular-nums`.

## 3. Spacing, bo góc, bóng
- Lưới 4px. Padding card 20–24px. Khoảng cách giữa các section 24–32px.
- Bo góc:
  - input và button 8px;
  - card 14px;
  - dialog 16px;
  - pill (badge, nút tròn) `rounded-full`.
- Bóng rất nhẹ: `shadow-xs` cho card; `shadow-lg` chỉ dùng cho dialog và popover. Không dùng viền đậm kết hợp với bóng đậm.

## 4. Component: chỉ dùng shadcn/ui + Lucide
- Thêm component bằng `npx shadcn@latest add <name>`. Không sửa trực tiếp `components/ui/*` trừ khi để chỉnh token. Biến thể mới viết bằng `cva`.
- Icon: chỉ dùng `lucide-react`, cỡ 16px (`size-4`), nét 2px. Nút chỉ có icon phải có `aria-label` và Tooltip.
- Mẫu đã chốt trong UI canvas:
  - **Sidebar:**
    - rộng 256px, thu gọn còn 56px;
    - có nhãn nhóm;
    - mỗi item cao 32px;
    - item active có nền `brand-soft` và chữ `brand`;
    - nút "Chat mới" cùng kiểu với item điều hướng.
  - **Avatar user ở góc dưới bên trái** mở DropdownMenu. Settings là **Dialog popup** có điều hướng bên trái, không phải một trang riêng.
  - **Header vùng nội dung:** cao 56px; nút thu gọn sidebar, separator dọc, Breadcrumb.
  - **Tabs:** nền `muted`, tab đang chọn nổi trắng.
  - **Badge trạng thái tài liệu:** mỗi trạng thái một màu cố định; dùng chung component `<DocStatusBadge>`.

## 5. Layout: vừa một màn hình, cuộn bên trong
- Khung app cao `h-dvh`, `overflow-hidden`. Sidebar đứng yên; chỉ vùng nội dung cuộn (`min-h-0 flex-1 overflow-y-auto`).
- Không để trang dài làm vỡ layout. Danh sách dài dùng cuộn trong khung; quá 200 dòng thì dùng virtualize (`@tanstack/react-virtual`).
- Màn Chat:
  - khung hội thoại cuộn riêng, rộng tối đa khoảng 820px;
  - ô nhập dính đáy;
  - panel Nguồn/Trace (360px) cuộn riêng và ẩn/hiện được.
- Thiết kế cho 1440×900, kiểm thêm ở 1280×800. Dưới 768px thì xếp dọc và cuộn cả trang.

## 6. Bốn trạng thái bắt buộc cho mọi khối dữ liệu
1. **Loading:** Skeleton đúng hình dạng nội dung thật, không dùng spinner toàn trang.
2. **Empty:** icon Lucide trong vòng tròn `muted`, một câu giải thích, một nút hành động chính.
3. **Error:** Alert `destructive`, thông điệp lấy từ mã lỗi (`errors.yaml`), có nút "Thử lại".
4. **Không có quyền:** ẩn hành động không được phép. Nếu cả màn bị cấm thì hiện thông báo, không để trống.

## 7. Motion
- Thời lượng 150–200ms, easing `cubic-bezier(0.2, 0, 0, 1)`.
- Chỉ animate `opacity` và `transform`.
- Dialog: fade cùng zoom 96→100%. Popover: fade cùng trượt 4px.
- Streaming câu trả lời: hiện từng token, con trỏ nhấp nháy. Không dùng hiệu ứng gõ giả.
- Tôn trọng `prefers-reduced-motion`.
- Hover trên hàng hoặc card chỉ đổi nền sang `muted/50`. Không phóng to.

## 8. Nội dung và vi mô (microcopy)
- Viết tiếng Việt ngắn, chủ động: "Tải tài liệu lên", không viết "Thực hiện việc tải lên tài liệu".
- Chuỗi đặt trong `messages/vi/<feature>.json` và `messages/en/<feature>.json`.
- Số và ngày định dạng theo locale (`Intl`).
- Hành động phá hủy (xóa KB, xóa tài liệu) dùng AlertDialog, nhắc tên đối tượng, nút `destructive`.

## 9. Accessibility (WCAG 2.2 AA)
- Độ tương phản ≥ 4.5:1 cho chữ thường.
- Thao tác được hoàn toàn bằng bàn phím; focus ring màu brand, dày 2px, có offset.
- Dialog giữ focus bên trong và trả focus về chỗ cũ khi đóng (Radix lo sẵn; không tự viết lại).
- Trạng thái realtime (tiến độ pipeline, streaming) dùng `aria-live="polite"`.

## 10. Tự kiểm trước khi báo xong
- [ ] Không có màu hex hay px lạ ngoài token và thang spacing.
- [ ] Đủ 4 trạng thái dữ liệu.
- [ ] Không có chuỗi hardcode; có đủ key vi/en.
- [ ] Không vỡ layout ở 1440×900 và 1280×800. Trang không sinh thanh cuộn ngoài ý muốn.
- [ ] Đi qua được bằng bàn phím; nút chỉ có icon có `aria-label`.
- [ ] Dark mode không có chữ chìm vào nền.
