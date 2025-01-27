# builder
FROM golang:1.23 AS builder

WORKDIR /build

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s" -a -installsuffix cgo -o /app/paste ./cmd/paste && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s" -a -installsuffix cgo -o /app/ping ./cmd/ping


# upx
FROM ubuntu:22.04 AS upx

RUN apt update -y && apt install -y --no-install-recommends upx

COPY --from=builder /app/ /app

RUN upx --best --no-lzma /app/*


# runtime
FROM scratch

COPY --from=upx /app/ /app

EXPOSE 80

HEALTHCHECK --interval=5s --timeout=10s --retries=3 CMD ["/app/ping", "http://localhost:80/ping/"]

CMD ["/app/paste"]
