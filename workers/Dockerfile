# Build: docker build -t rag-worker workers/
# Run:   docker run --rm --env-file deployment/.env rag-worker --stage parse

# ---------- build: cài deps vào .venv bằng uv ----------
FROM python:3.12-slim AS build
COPY --from=ghcr.io/astral-sh/uv:0.12.12 /uv /bin/uv

ENV UV_COMPILE_BYTECODE=1 \
    UV_LINK_MODE=copy \
    UV_PYTHON_DOWNLOADS=never

WORKDIR /app

# Deps trước, code sau: đổi code không phải cài lại deps
RUN --mount=type=cache,target=/root/.cache/uv \
    --mount=type=bind,source=uv.lock,target=uv.lock \
    --mount=type=bind,source=pyproject.toml,target=pyproject.toml \
    uv sync --locked --no-dev --no-install-project

COPY pyproject.toml uv.lock ./
COPY rag_worker ./rag_worker
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --locked --no-dev --no-editable

# ---------- runtime: không có uv, không có source ngoài .venv ----------
FROM python:3.12-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PATH="/app/.venv/bin:$PATH"

RUN useradd --uid 10001 --no-create-home --shell /usr/sbin/nologin worker

WORKDIR /app
COPY --from=build --chown=worker:worker /app/.venv /app/.venv

USER worker
ENTRYPOINT ["python", "-m", "rag_worker"]
