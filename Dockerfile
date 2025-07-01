# builder
FROM golang:1.23 AS builder

ARG APP_VERSION=not-set

WORKDIR /build

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    OUTPUTDIR="/app/" APP_VERSION=$APP_VERSION make build


# upx
FROM ubuntu:22.04 AS upx

RUN apt-get update -y && apt-get install -y --no-install-recommends upx

COPY --from=builder /app/ /app

RUN upx --best --no-lzma /app/*


# runtime
FROM scratch

COPY --from=upx /app/ /app

EXPOSE 80

CMD ["/app/paste", "--host", "0.0.0.0", "--port", "80", "--dbport", "6379", "--health"]
