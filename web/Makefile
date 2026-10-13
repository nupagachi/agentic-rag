# web/ - Next.js 15 app. Contract: see "Them ung dung/area" in the root README.
# Recipes are ASCII only (Windows make mangles UTF-8).

.PHONY: check dev sec

check: ## Lint + typecheck + unit tests + production build (no .env needed)
	npm ci --no-fund --no-audit
	npm run lint
	npm run typecheck
	npx vitest run
	npm run build

dev: ## Run Next.js dev server on 127.0.0.1:3000 (loads ../.env)
	@test -f ../.env || { echo "missing ../.env: run 'make env' at the repo root" >&2; exit 1; }
	@set -a; . ../.env; set +a; exec npm run dev

# Runtime dependencies: fail on high or critical (what ships to users).
# Full tree incl. dev tools: fail on critical only, because eslint-config-next ->
# @next/eslint-plugin-next -> fast-glob -> micromatch -> braces (GHSA-vfj7-8cjw-p6xm,
# high, dev-only, no patched braces release exists as of Next 15.5.27 / 16.4.0).
# TODO(M0-T04): tighten the second command to --audit-level=high once upstream ships a fix.
sec: ## npm audit on the lockfile (runtime: high, full tree: critical)
	npm audit --omit=dev --audit-level=high
	npm audit --audit-level=critical
