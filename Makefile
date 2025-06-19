test:
	GOMAXPROCS=1 go test -tags integration,e2e,e2ettl -count=1 -race -cover -covermode=atomic ./... && \
	go tool cover -func=coverage.out

build:
	go generate ./... && \
	CGO_ENABLED=0 go build -v ./...
