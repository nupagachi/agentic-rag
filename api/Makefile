# Go API. Contract: see "Them ung dung/area" in ../README.md (targets: check, dev, sec).
# Keep recipes ASCII only: make on Windows corrupts UTF-8 passed to bash.
#
# Tool versions are pinned and must match ../mise.toml. They run through
# `go run <module>@<version>`, so no global install is needed. Go itself is
# selected by go.mod (go 1.27 + toolchain directive, GOTOOLCHAIN=auto).

GOLANGCI_LINT_VERSION := 2.14.0
GOVULNCHECK_VERSION   := 1.9.0
AIR_VERSION           := 1.67.4

GOLANGCI_LINT ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v$(GOLANGCI_LINT_VERSION)
GOVULNCHECK   ?= go run golang.org/x/vuln/cmd/govulncheck@v$(GOVULNCHECK_VERSION)
AIR           ?= go run github.com/air-verse/air@v$(AIR_VERSION)

# The race detector needs cgo (a C compiler). Use it when available (CI, Linux).
RACE := $(if $(filter 1,$(shell go env CGO_ENABLED)),-race,)

.PHONY: check dev sec

check:
	@echo "api: gofmt"
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt: these files need formatting:" >&2; echo "$$out" >&2; exit 1; fi
	@echo "api: go mod verify / tidy"
	@go mod verify
	@go mod tidy -diff
	@echo "api: go vet"
	@go vet ./...
	@echo "api: golangci-lint"
	@$(GOLANGCI_LINT) run ./...
	@echo "api: go test $(RACE)"
	@go test $(RACE) ./...

dev:
	@test -f ../.env || { echo "missing ../.env: run 'make env' at the repo root" >&2; exit 1; }
	@set -a; . ../.env; set +a; exec $(AIR) -c .air.toml

sec:
	@echo "api: govulncheck"
	@$(GOVULNCHECK) ./...
