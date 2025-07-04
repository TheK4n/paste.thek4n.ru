APP_VERSION ?= not-set
OUTPUTDIR ?= bin/

default: build

.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux \
	go build -v \
		-ldflags "-w -s -X 'main.version=$(APP_VERSION)'" \
		-o $(OUTPUTDIR) ./...

.PHONY: e2e
e2e:
	GOMAXPROCS=1 \
	go test \
		-tags e2e \
		-failfast \
		-count=1 \
		./...

.PHONY: e2e-short
e2e-short:
	GOMAXPROCS=1 \
	go test \
		-tags e2e \
		-short \
		-failfast \
		-count=1 \
		./...

.PHONY: cover
.ONESHELL:
cover:
	cover_profile=$$(mktemp)
	GOMAXPROCS=1 \
	go test \
		-tags integration,e2e \
		-failfast \
		-count=1 \
		-cover -covermode=atomic \
		-coverprofile="$$cover_profile" \
		./...
	go tool cover -func="$$cover_profile"
	rm "$$cover_profile"

.PHONY: integration
integration:
	GOMAXPROCS=1 \
	go test \
		-tags integration \
		-failfast \
		-count=1 \
		./...

.PHONY: test
test:
	GOMAXPROCS=1 \
	go test \
		-tags integration,e2e \
		-failfast \
		-count=1 \
		./...

.PHONY: lint
lint:
	GOFLAGS="-tags=integration,e2e" \
	golangci-lint run --fix --timeout=5m

.PHONY: lint-short
lint-short:
	GOFLAGS="-tags=integration,e2e" \
	golangci-lint run --fix --new-from-rev HEAD --timeout=5m


.PHONY: fmt
fmt:
	go fmt ./...
	GOFLAGS="-tags=integration,e2e" \
	go vet ./...
