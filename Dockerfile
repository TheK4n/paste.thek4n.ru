# builder
FROM golang:1.23 AS builder

WORKDIR /build

RUN --mount=type=bind,source=go.mod,target=go.mod \
    --mount=type=bind,source=go.sum,target=go.sum \
    go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s" -a -installsuffix cgo -o /app/paste ./cmd/paste


# runtime
FROM scratch

COPY --from=builder /app/paste /app/paste

EXPOSE 80

CMD ["/app/paste"]
