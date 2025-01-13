# builder
FROM golang:1.23 AS builder

WORKDIR /app

COPY . .

RUN go get -v ./...
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s" -a -installsuffix cgo -o app ./cmd


# runtime
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/app .

CMD ["./app"]
