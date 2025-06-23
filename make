#!/bin/sh

set -ue


cmd_test() {
    GOMAXPROCS=1 \
        go test \
            -tags integration,e2e \
            -count=1 \
            -race \
            -cover -covermode=atomic \
            ./...
}

cmd_testall() {
    cover_profile_file="$(mktemp)"
    GOMAXPROCS=1 \
        go test \
            -tags integration,e2e,e2ettl \
            -count=1 \
            -race \
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
    testall) shift;  cmd_testall  "${@}" ;;
    test) shift;     cmd_test  "${@}" ;;
    build) shift;    cmd_build "${@}" ;;

    *) echo "No specified command ${*}" 1>&2 ;;
esac
