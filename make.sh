#!/bin/sh

set -ue


test() {
    export GOMAXPROCS=1
    go test -tags integration,e2e,e2ettl -count=1 -race -cover -coverprofile=/tmp/coverage.out -covermode=atomic ./... && \
    go tool cover -func=/tmp/coverage.out
}

build() {
    export CGO_ENABLED=0
    export GOOS=linux

    go generate ./... && \
    go build -v -ldflags "-w -s" -a -installsuffix cgo "$@" ./...
}

func="${1}"; shift
"${func}" "$@"
