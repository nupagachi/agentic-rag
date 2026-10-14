# Makefile of the Python worker. Called from the repo root: `make check-workers`,
# `make dev-worker`, `make sec`. Contract: see README "Them ung dung/area".
# Recipes are ASCII only (make on Windows mangles UTF-8 passed to bash).

SHELL := bash
.DEFAULT_GOAL := check

.PHONY: check dev sec test-integration

# Lint + typecheck + unit tests. Needs no Docker and no .env.
check:
	uv lock --check
	uv sync --frozen
	uv run --frozen ruff check .
	uv run --frozen ruff format --check .
	uv run --frozen mypy --strict rag_worker
	uv run --frozen pytest -m "not integration"

# Hot reload: restart on any .py change. Loads ../.env (made by `make env` at the repo root).
dev:
	@test -f ../.env || { echo "missing ../.env: run 'make env' at the repo root" >&2; exit 1; }
	@uv sync --frozen
	@set -a; . ../.env; set +a; exec uv run --frozen watchfiles --filter python --args "--stage idle" rag_worker.__main__.main rag_worker

# Dependency audit against the lockfile. Fail-closed: pip-audit's exit code is propagated.
sec:
	uv sync --frozen
	uv run --frozen pip-audit

# Integration tests against the dev stack from `make up`. Not part of `check`.
# Loads ../.env when present; tests that need Redis skip with a reason when it is not
# running (set INTEGRATION_REQUIRE_STACK=1 to fail instead). -rs lists the skip reasons.
test-integration:
	@echo "workers: integration tests (stack from 'make up', settings from ../.env)"
	@if [ -f ../.env ]; then set -a; . ../.env; set +a; fi; \
	uv sync --frozen && uv run --frozen pytest -m integration -rs tests/integration
