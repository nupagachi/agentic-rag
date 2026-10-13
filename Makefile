# Makefile gốc của monorepo Agentic RAG.
#
# Quy ước:
#  - Makefile này chỉ ĐIỀU PHỐI: lệnh theo ứng dụng nằm ở <app>/Makefile
#    (check, dev, sec; tùy chọn test-integration). Xem README mục "Thêm ứng dụng/area".
#  - Chạy được bằng GNU make 3.81+ (macOS) và make trong Git Bash trên Windows.
#    Không dùng .ONESHELL, .SHELLFLAGS, grouped targets hay lệnh PowerShell.
#  - `make help` tự sinh từ comment `## mô tả` ở cuối dòng khai báo target.
#  - Recipe CHỈ chứa ASCII: make bản Windows làm hỏng ký tự UTF-8 khi truyền lệnh cho bash.
#    Thông điệp tiếng Việt nằm ở các dòng `#msg:<khóa>=<nội dung>` cuối file và được in
#    bằng macro `say` (đọc thẳng từ file nên giữ nguyên UTF-8).

SHELL := bash
.DEFAULT_GOAL := help

PROJECT      := agentic-rag-dev
COMPOSE_FILE := deploy/docker-compose.dev.yml
ENV_EXAMPLE  := deploy/.env.example

# $(call say,<khóa>,<đối số printf>): in thông điệp #msg:<khóa> (không tự xuống dòng).
define say
printf "$$(sed -n 's/^[#]msg:$(1)=//p' $(firstword $(MAKEFILE_LIST)))" $(2)
endef

# Lệnh docker compose cho project dev. Chọn .env nếu đã có, ngược lại dùng
# .env.example (chỉ để down/ps/config xác định được project; `up` luôn chạy sau `env`).
# $(call compose,<lệnh con và cờ>)
define compose
docker compose -p $(PROJECT) -f $(COMPOSE_FILE) --env-file "$$([ -f .env ] && echo .env || echo $(ENV_EXAMPLE))" $(1)
endef

# Ủy quyền cho Makefile con. App chưa có Makefile thì bỏ qua (mã 0) để main xanh
# giữa các wave; đã có Makefile mà lỗi thì lỗi được truyền lên (fail).
# $(call delegate,<thư mục app>,<target>)
define delegate
@if [ -f $(1)/Makefile ]; then $(MAKE) --no-print-directory -C $(1) $(2); else $(call say,skip,$(1)); fi
endef

.PHONY: help env up down reset ps dev-api dev-worker dev-web \
        check check-api check-workers check-web \
        sec sec-tools sec-compose sec-secrets sec-apps

help: ## Liệt kê các lệnh (mặc định)
	@awk 'BEGIN { FS = ":.*## " } /^[a-zA-Z0-9_-]+:.*## / { printf "  %-14s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# ---------------------------------------------------------------- môi trường dev

env: ## Tạo .env từ deploy/.env.example nếu chưa có (không ghi đè)
	@if [ -f .env ]; then \
		$(call say,env_exists,); \
	else \
		cp $(ENV_EXAMPLE) .env && $(call say,env_created,$(ENV_EXAMPLE)); \
	fi

up: env ## Chạy Postgres 16 + Redis 7.2 (chỉ bind 127.0.0.1), chờ healthy
	@$(call compose,up -d --wait)

down: ## Dừng Postgres + Redis, giữ dữ liệu (volume)
	@$(call compose,down)

ps: ## Xem trạng thái Postgres + Redis
	@$(call compose,ps)

reset: ## Xóa dữ liệu dev của project này (hỏi y/N) rồi chạy lại up
	@$(call say,reset_ask,$(PROJECT)); \
	read -r ans; \
	if [ "$$ans" = "y" ] || [ "$$ans" = "Y" ]; then \
		$(call compose,down -v) && $(MAKE) --no-print-directory up; \
	else \
		$(call say,reset_cancel,); \
	fi

# ---------------------------------------------------------------- chạy ứng dụng

dev-api: ## Chạy Go API với hot reload (api/Makefile: dev)
	$(call delegate,api,dev)

dev-worker: ## Chạy Python worker với hot reload (workers/Makefile: dev)
	$(call delegate,workers,dev)

dev-web: ## Chạy Next.js dev server tại 127.0.0.1 (web/Makefile: dev)
	$(call delegate,web,dev)

# ---------------------------------------------------------------- kiểm tra chất lượng

check: check-api check-workers check-web ## Lint, typecheck, test cho cả 3 ứng dụng

check-api: ## Kiểm tra Go API (api/Makefile: check)
	$(call delegate,api,check)

check-workers: ## Kiểm tra Python worker (workers/Makefile: check)
	$(call delegate,workers,check)

check-web: ## Kiểm tra Next.js (web/Makefile: check)
	$(call delegate,web,check)

# ---------------------------------------------------------------- bảo mật (fail-closed)
# Không nuốt mã lỗi của bất kỳ bước quét nào (không tiền tố trừ, không toán tử "hoặc true");
# công cụ lỗi, thiếu hoặc có phát hiện thì `make sec` trả mã khác 0.

sec: sec-tools sec-compose sec-secrets sec-apps ## Quét bảo mật: gitleaks, kiểm compose, rồi sec của từng app

sec-tools:
	@command -v gitleaks >/dev/null 2>&1 || { \
		$(call say,err_gitleaks,"$$(sed -n 's/^gitleaks *= *"\(.*\)"/\1/p' mise.toml)") >&2; \
		exit 1; }
	@command -v docker >/dev/null 2>&1 || { $(call say,err_docker,) >&2; exit 1; }

# SR-1, SR-2, SR-3: ports chỉ trên 127.0.0.1, không trust, Redis có mật khẩu, image pin patch.
sec-compose: sec-tools
	@set -euo pipefail; \
	cfg=$$(docker compose -p $(PROJECT) -f $(COMPOSE_FILE) --env-file $(ENV_EXAMPLE) config | tr -d '\r'); \
	imgs=$$(docker compose -p $(PROJECT) -f $(COMPOSE_FILE) --env-file $(ENV_EXAMPLE) config --images | tr -d '\r'); \
	pub=$$(printf '%s\n' "$$cfg" | awk '/^[[:space:]]+published:/ { n++ } END { print n + 0 }'); \
	loop=$$(printf '%s\n' "$$cfg" | awk '/^[[:space:]]+host_ip: 127\.0\.0\.1$$/ { n++ } END { print n + 0 }'); \
	if [ "$$pub" -lt 1 ] || [ "$$pub" -ne "$$loop" ]; then \
		$(call say,err_sr1,"$$pub" "$$loop") >&2; exit 1; fi; \
	if grep -qi 'HOST_AUTH_METHOD' $(COMPOSE_FILE); then \
		$(call say,err_sr2_trust,$(COMPOSE_FILE)) >&2; exit 1; fi; \
	if ! grep -q -- '--requirepass' $(COMPOSE_FILE); then \
		$(call say,err_sr2_redis,) >&2; exit 1; fi; \
	if [ -z "$$imgs" ]; then $(call say,err_sr3_none,) >&2; exit 1; fi; \
	for img in $$imgs; do \
		case "$$img" in *latest*) $(call say,err_sr3_latest,"$$img") >&2; exit 1;; esac; \
		if ! printf '%s' "$$img" | grep -Eq ':[0-9]+\.[0-9]+(\.[0-9]+)?'; then \
			$(call say,err_sr3_tag,"$$img") >&2; exit 1; fi; \
	done; \
	redis_patch=$$(printf '%s\n' "$$imgs" | sed -n 's/^redis:7\.2\.\([0-9][0-9]*\).*/\1/p'); \
	if [ -z "$$redis_patch" ] || [ "$$redis_patch" -lt 11 ]; then \
		$(call say,err_sr3_redis,) >&2; exit 1; fi; \
	$(call say,ok_compose,"$$pub")

# Quét lịch sử và cây làm việc. Cây làm việc chỉ gồm file mà git thấy (đã track, hoặc chưa
# track nhưng không bị ignore); sao ra thư mục tạm để không quét node_modules, .venv...
sec-secrets: sec-tools
	@set -euo pipefail; \
	gitleaks git --config .gitleaks.toml --redact --verbose --no-banner .; \
	tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT; \
	git ls-files -z --cached --others --exclude-standard \
		| while IFS= read -r -d '' f; do if [ -f "$$f" ]; then printf '%s\0' "$$f"; fi; done \
		| tar --null -T - -cf - | tar -xf - -C "$$tmp"; \
	gitleaks dir --config .gitleaks.toml --redact --verbose --no-banner "$$tmp"

sec-apps:
	$(call delegate,api,sec)
	$(call delegate,workers,sec)
	$(call delegate,web,sec)

# ---------------------------------------------------------------- thông điệp (UTF-8)
#msg:skip=skip: %s chưa có\n
#msg:env_exists=.env đã có, giữ nguyên\n
#msg:env_created=đã tạo .env từ %s (giá trị chỉ dùng cho dev)\n
#msg:reset_ask=Sẽ XÓA toàn bộ dữ liệu (volume) của project "%s" rồi chạy lại. Tiếp tục? [y/N]\040
#msg:reset_cancel=đã hủy, không xóa gì\n
#msg:err_gitleaks=lỗi: thiếu gitleaks. Cài bằng 'mise install' (xem README, mục Cài đặt) hoặc cài thủ công đúng phiên bản trong mise.toml (%s).\n
#msg:err_docker=lỗi: thiếu docker (cần để kiểm compose). Cài Docker Desktop, xem README.\n
#msg:err_sr1=lỗi SR-1: %s cổng published nhưng chỉ %s cổng bind 127.0.0.1 (cần bằng nhau và >= 1)\n
#msg:err_sr2_trust=lỗi SR-2: %s không được đặt POSTGRES_HOST_AUTH_METHOD\n
#msg:err_sr2_redis=lỗi SR-2: Redis phải chạy với --requirepass\n
#msg:err_sr3_none=lỗi SR-3: không đọc được danh sách image\n
#msg:err_sr3_latest=lỗi SR-3: image dùng latest: %s\n
#msg:err_sr3_tag=lỗi SR-3: image thiếu tag phiên bản cụ thể: %s\n
#msg:err_sr3_redis=lỗi SR-3: cần redis:7.2.x với x >= 11 (CVE-2025-49844)\n
#msg:ok_compose=ok: compose (%s cổng đều ở 127.0.0.1; không trust; Redis có mật khẩu; image đã pin)\n
