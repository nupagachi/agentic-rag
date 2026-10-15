# syntax=docker/dockerfile:1
# Build: docker build -t rag-api ./server
# Run:   docker run --rm -p 8080:8080 --env-file server/.env rag-api

FROM golang:1.26 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/app

# Image tĩnh, không shell, chạy user nonroot (uid 65532)
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
# Khi có migrations (M0-05): COPY migrations /migrations

ENV APP_ENV=production \
    HTTP_ADDR=:8080 \
    LOG_FORMAT=json \
    GIN_MODE=release
EXPOSE 8080
USER nonroot:nonroot

# distroless không có curl: dùng chính binary để kiểm tra /healthz
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app", "healthcheck"]

ENTRYPOINT ["/app"]
CMD ["api"]
