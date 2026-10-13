---
name: be-python-standards
description: Convention Python 3.12 cho workers/ - stage idempotent run(job, ctx), Pydantic từ contracts, retry/DLQ, xử lý file không tin cậy, gọi model qua gateway, typing strict, ruff, mypy, pytest. Dùng khi viết hoặc review code Python.
user-invocable: false
paths: workers/**
---

# Convention Python (workers/)

Nền tảng: PEP 8, PEP 484/695 typing, Google Python Style Guide. Phần dưới là quy định riêng của dự án.

## 1. Stage
```python
# rag_worker/stages/chunk.py
async def run(job: ChunkJob, ctx: StageContext) -> EmbedJob | None:
    ...
```
- Mỗi stage là một hàm `run(job, ctx)`. Các việc nhận job, ack, retry, DLQ, heartbeat và OTel nằm trong `runtime/`; stage không tự làm.
- **Idempotent**: chạy lại cùng `(document_id, version, stage)` phải cho cùng kết quả.
  - Ghi Postgres bằng upsert.
  - Point ID của Qdrant là `uuid5(NAMESPACE, f"{doc_id}:{version}:{chunk_idx}")`.
  - Object MinIO có key xác định trước.
- Phân loại lỗi:
  - `RetryableError` cho mạng, 429, timeout: runtime retry với backoff, tối đa 5 lần.
  - `PermanentError` cho file hỏng, định dạng không hỗ trợ: chuyển sang DLQ và đặt trạng thái `Failed` kèm mã lỗi trong `errors.yaml`.
- Mỗi bước ghi một sự kiện qua `ctx.events.emit(stage, status, detail)` để Inspector hiển thị.

## 2. Hợp đồng
- Model job và event là Pydantic v2 **sinh từ** `contracts/jobs/*.schema.json` vào `rag_worker/contracts/`. Không sửa tay.
- Validate job ngay khi nhận. Job sai schema thì chuyển thẳng vào DLQ.

## 3. File không tin cậy
- Quét ClamAV trước khi parse.
- Kiểm MIME bằng `libmagic`, không tin đuôi file.
- Giới hạn:
  - kích thước;
  - số trang;
  - tỉ lệ giải nén (chống zip bomb);
  - thời gian parse mỗi tài liệu (timeout, chạy trong process con có giới hạn bộ nhớ).
- Đọc file lớn theo dạng stream, không nạp cả file vào RAM. File tạm nằm trong `tempfile.TemporaryDirectory()` và luôn được dọn.
- Crawl chỉ đi qua proxy (`HTTPS_PROXY` cổng 3129). Không tắt kiểm TLS. Redirect phải kiểm lại scheme và host.

## 4. Gọi model
- Chỉ dùng `clients/gateway.py` (httpx async, có timeout, header `X-Internal-Token`). Worker **không** giữ API key.
- Embed theo batch đúng giới hạn của model. Đếm token bằng `tiktoken` trước khi gửi.

## 5. Code style
- Bật `from __future__ import annotations`; type đầy đủ; `mypy --strict` phải sạch.
- `ruff check` (bộ quy tắc E, F, I, B, UP, S, SIM, RUF, ASYNC) + `ruff format`.
- I/O dùng async (`asyncio`, `httpx`, `psycopg` async). Việc nặng CPU (parse) chạy trong `ProcessPoolExecutor` có giới hạn.
- Cấu hình bằng `pydantic-settings`, đọc từ biến môi trường, validate khi khởi động.
- Log bằng `structlog` dạng JSON, có `trace_id`, `job_id`, `document_id`. Không log nội dung tài liệu ở mức info.
- Không dùng `print`, không `except Exception: pass`, không dùng tham số mặc định kiểu mutable.

## 6. Tokenizer BM25
`textproc/bm25.py` phải cho kết quả **giống hệt** bản Go. Mọi thay đổi phải pass `contracts/bm25/test_vectors.json`, và nếu đổi đặc tả thì cập nhật cả hai ngôn ngữ trong cùng một PR contract.

## 7. Test
- `pytest` + `pytest-asyncio`. Fixture tài liệu lấy từ `testdata/` (PDF scan, bảng phức tạp, PPTX, trang JS).
- Unit test cho parser và chunker: kiểm số chunk, ranh giới heading, overlap, số trang.
- Test tích hợp dùng testcontainers (Postgres, Redis, Qdrant, MinIO) và mock gateway.
- Test idempotency: chạy một stage hai lần, trạng thái cuối phải giống nhau và không sinh bản ghi trùng.
- Coverage của `stages/` và `chunkers/` ≥ 80%.

## 8. Lệnh
`make check-workers` = `uv run ruff check` + `uv run ruff format --check` + `uv run mypy --strict rag_worker` + `uv run pytest -q`.
