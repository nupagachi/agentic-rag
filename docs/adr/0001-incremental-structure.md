# ADR 0001: Cấu trúc repo tăng dần, M0 chỉ chạy local

- Trạng thái: Đề xuất (chờ Owner duyệt cùng SPEC-M0-platform)
- Ngày: 2026-10-13
- Liên quan: `docs/specs/SPEC-M0-platform.md` (mục 2, 3, 10), `docs/Kiến trúc.md` (mục 3)

## Bối cảnh

- Repo mới chỉ có tài liệu và bộ cấu hình agent. Cần một khung để ba ứng dụng (`web`, `api`, `workers`), Postgres và Redis chạy được trên máy dev và có cổng kiểm `make check`.
- Cấu trúc đích trong `docs/Kiến trúc.md` mục 3 rất lớn (contracts, migration, gateway LLM, retrieval, eval, observability...). Dựng hết từ đầu tốn công và dễ sai, vì nhiều quyết định chỉ rõ khi làm tới tính năng thật. Thư mục rỗng và cấu hình không ai dùng cũng dễ lỗi thời mà không bị phát hiện.
- Chưa có runner CI/CD hay máy chủ. Hạ tầng triển khai chưa có người dùng thật nên chưa kiểm chứng được.

## Quyết định

1. **M0 chỉ tạo phần mọi module sau đều cần ngay:** thư mục ba ứng dụng, bộ lệnh `make` (`env`, `up`, `down`, `reset`, `dev-*`, `check*`, `sec`), lint/test, Postgres 16 + Redis 7.2 qua `deploy/docker-compose.dev.yml`, toolchain pin trong `mise.toml`, README, ADR này.
2. **Mọi cấu trúc còn lại do module đầu tiên cần nó thêm vào**, trong cùng PR với tính năng đó và chỉ ở mức tính năng đó dùng (thư mục, thư viện, service Docker, file contract, lệnh `make`).
3. **Đích cuối không đổi:** vẫn là cấu trúc ở `docs/Kiến trúc.md` mục 3. Module thêm cấu trúc phải đặt đúng vị trí và tên đã ghi ở đó, không tự đặt chỗ khác.
4. **Task đầu tiên của mỗi module** (thường ở wave 0) gồm luôn phần cấu trúc mới theo bảng gợi ý ở SPEC-M0 mục 2.
5. **Makefile gốc chỉ ủy quyền** cho `<app>/Makefile` (`check`, `dev`, `sec`; tùy chọn `test-integration`). App chưa có Makefile thì bỏ qua với mã 0; đã có thì lỗi phải làm fail. Nhờ vậy các app ở cùng wave không sửa chung file.
6. **M0 chỉ chạy local.** Repo đẩy lên GitHub để làm PR, nhưng chưa có Actions; cổng kiểm trước PR là `make check` và `make sec` chạy trên máy.
7. **Không tạo trước** thư mục rỗng hay `.gitkeep`.

### Danh sách hoãn (SPEC-M0 mục 10)

Agent không làm các hạng mục sau trong M0:

| Hạng mục | Làm khi |
| --- | --- |
| CI/CD, Dockerfile production, registry, compose production 2 máy | Có runner / máy chủ |
| Nginx + TLS, Squid egress proxy | Trước khi gọi provider thật (M3) hoặc upload presigned (M4) |
| Prometheus, Grafana/Loki/Tempo | Khi có staging |
| Backup, alert, runbook | Có máy chủ |

Cũng hoãn theo bảng module của SPEC-M0 mục 2: `contracts/` và `make gen` (M1), migration và sqlc (M1), `internal/llm/*` (M3), runtime worker, MinIO, Qdrant, ClamAV (M3-M4)...

## Hệ quả

Tích cực:
- Diff của M0 nhỏ, dễ review; mỗi cấu trúc ra đời cùng tính năng dùng nó nên được thiết kế với yêu cầu thật.
- Dev mới chạy được khung trong thời gian ngắn (mục tiêu ≤ 10 phút) mà không phải hiểu phần chưa dùng.
- Ba app ở wave 1 làm song song không đụng file chung (mỗi app một Makefile).

Tiêu cực, rủi ro:
- Mỗi module phải tự thêm cấu trúc, nên cần kỷ luật đặt đúng vị trí theo `docs/Kiến trúc.md`; reviewer phải kiểm.
- Task đầu của module có thể lớn hơn vì kèm phần cấu trúc; orchestrator cần tách task khi vượt cỡ.
- Một số quyết định hạ tầng (TLS, egress proxy) bị đẩy muộn; phải nhớ làm đúng thời điểm trong bảng hoãn, nhất là trước khi gọi LLM provider thật.
- Compose dev và `mise.toml` pin phiên bản cụ thể nên cần nâng định kỳ (bản vá bảo mật) bằng PR riêng.

## Điều kiện xem lại

Xem lại ADR này khi một trong các điều kiện sau xảy ra:
- Có runner CI/CD hoặc máy chủ staging/production (mở các hạng mục hoãn).
- Nhiều module phải chờ nhau vì thiếu cấu trúc nền mà module trước "chưa cần" (dấu hiệu chia sai thời điểm).
- Hai module trở lên tự đặt cùng một loại cấu trúc ở chỗ khác nhau, hoặc lệch khỏi `docs/Kiến trúc.md` mục 3.
- Owner quyết định chuyển cách chạy ứng dụng (vd. chạy trong compose thay vì trên host) hoặc bắt buộc WSL2 cho Windows (SPEC-M0 mục 9, câu hỏi 1 và 2).
