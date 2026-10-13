---
id: M1-T03
spec: SPEC-M1-auth
p0_ref: M1-03
title: API đăng nhập - JWT access RS256 + refresh cookie httpOnly
owner: be-agent            # fe-agent | be-agent | test-agent
type: feat                 # feat | fix | test | refactor | chore
area: api                  # web | api | workers | contracts | deploy
wave: 1
depends_on: [M1-T01, M1-T02]
branch: feat/M1-T03-login-api
scope:
  paths:
    - api/internal/auth/**
    - api/queries/auth.sql
  forbidden:
    - contracts/**
    - api/internal/authz/**
size: M                    # S ≤150 dòng | M ≤400 dòng
allowed_new_deps: []       # ví dụ: [github.com/golang-jwt/jwt/v5]
status: todo               # todo | ready | in-progress | in-review | changes-requested | merged | blocked
pr: null
---

## Mục tiêu
Một câu: sau task này, hệ thống làm được gì.

## Bối cảnh và tham chiếu
- SPEC: `docs/specs/SPEC-M1-auth.md` mục 3.2
- Contract: `contracts/openapi.yaml#/paths/~1auth~1login`
- Kiến trúc: `docs/Kiến trúc.md` mục 3 (Go API → auth/)
- Code mẫu nên bắt chước: `api/internal/account/`

## Acceptance criteria
- **AC1**: Given email và mật khẩu đúng, When POST `/api/v1/auth/login`, Then trả 200 với `accessToken` (RS256, exp 15 phút) và set cookie refresh `HttpOnly; Secure; SameSite=Strict`.
- **AC2**: Given mật khẩu sai, Then trả 401 `AUTH_INVALID_CREDENTIALS`. Thông điệp không cho biết email có tồn tại hay không.
- **AC3**: …

## Yêu cầu bảo mật (từ threat model)
- **SR-2**: So sánh mật khẩu bằng argon2id verify (constant-time); thời gian phản hồi khi email không tồn tại tương đương email có tồn tại.
- **SR-5**: Không log email hay mật khẩu ở mức info.

## Yêu cầu test
- Unit: service Login (đúng, sai mật khẩu, user bị khóa, user chưa xác thực email).
- Tích hợp: handler + Postgres testcontainer.

## Ngoài phạm vi
- Rate limit (M1-T07), refresh rotation (M1-T04), UI (M1-T09).

## Ghi chú cho agent
- …
