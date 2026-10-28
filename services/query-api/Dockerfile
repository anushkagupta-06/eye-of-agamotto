FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY services/go.mod services/go.sum ./
RUN go mod download

COPY services/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /query-api ./query-api/cmd/query-api

FROM alpine:3.20
RUN apk --no-cache add ca-certificates curl tzdata

WORKDIR /app
COPY --from=builder /query-api /app/query-api

EXPOSE 8082

HEALTHCHECK --interval=5s --timeout=3s --retries=10 \
  CMD curl -fsS http://127.0.0.1:8082/readyz || exit 1

ENTRYPOINT ["/app/query-api"]
