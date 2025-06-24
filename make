#!/bin/sh

set -ue


cmd_testshort() {
    GOMAXPROCS=1 \
        go test \
            -tags integration,e2e \
            -short \
            -failfast \
            -count=1 \
            ./...
}

cmd_testall() {
    cover_profile_file="$(mktemp)"
    GOMAXPROCS=1 \
        go test \
            -tags integration,e2e \
            -failfast \
            -count=1 \
            -cover -covermode=atomic \
            -coverprofile="${cover_profile_file}" \
            ./... && \
    go tool cover -func="${cover_profile_file}"
    rm "${cover_profile_file}"
}

cmd_build() {
    output="${OUTPUTDIR:-bin/}"
    version="${APP_VERSION:-not-set}"

    CGO_ENABLED=0 \
    GOOS=linux \
        go build -v -ldflags "-w -s -X 'main.version=${version}'" -o "${output}" ./...
}


if [ -z "${1+x}" ]; then
    cmd_build
    exit 0
fi

case "${1}" in
    test) shift;      cmd_testall  "${@}" ;;
    testshort) shift; cmd_testshort  "${@}" ;;
    build) shift;     cmd_build "${@}" ;;

    *) echo "No specified command ${*}" 1>&2 ;;
esac
