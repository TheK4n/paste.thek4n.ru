#!/bin/sh

set -ue


cmd_test() {
    GOMAXPROCS=1 \
        go test -tags integration,e2e,e2ettl -count=1 -race -cover -covermode=atomic ./...
}

cmd_build() {
    output="${OUTPUTDIR:-bin/}"
    version="${APP_VERSION:-unknown}"

    CGO_ENABLED=0 \
    GOOS=linux \
        go build -v -ldflags "-w -s -X 'main.version=${version}'" -o "${output}" ./...
}

case "${1}" in
    test) shift;  cmd_test  "$@" ;;
    build) shift; cmd_build "$@" ;;
esac
