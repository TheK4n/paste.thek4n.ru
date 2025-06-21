#!/bin/sh

set -ue


test() {
    export GOMAXPROCS=1
    go test -tags integration,e2e,e2ettl -count=1 -race -cover -covermode=atomic ./...
}

build() {
    export CGO_ENABLED=0
    export GOOS=linux

    version="$(git describe --tags --abbrev=0)"
    go build -v -ldflags "-w -s -X 'main.version=${version}'" "$@" ./...
}

func="${1}"; shift

"${func}" "$@"
