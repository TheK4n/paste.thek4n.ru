# builder
FROM golang:1.24 AS builder

ARG APP_VERSION=not-set

WORKDIR /build

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    OUTPUTDIR="/app/" APP_VERSION=$APP_VERSION make build


# runtime
FROM scratch

COPY --from=builder /app/ /app

EXPOSE 80

CMD ["/app/paste", "run", "--host", "0.0.0.0", "--port", "80", "--dbport", "6379", "--health", "--docs"]
