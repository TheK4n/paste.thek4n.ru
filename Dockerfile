# builder
FROM golang:1.23 AS builder

WORKDIR /build

COPY go.mod go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s" -a -installsuffix cgo -o paste-service ./cmd


# runtime
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl

COPY --from=builder /build/paste-service /bin

EXPOSE 80

CMD ["paste-service"]
