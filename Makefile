APP_VERSION ?= built-from-source
OUTPUTDIR ?= bin/

default: build

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
		-tags frontend,integration,e2e \
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

.PHONY: unit
unit:
	GOMAXPROCS=1 \
	go test \
		-tags unit \
		-failfast \
		-count=1 \
		./...

.PHONY: unit-short
unit-short:
	GOMAXPROCS=1 \
	go test \
		-tags unit \
		-short \
		-failfast \
		-count=1 \
		./...

.PHONY: test
test:
	GOMAXPROCS=1 \
	go test \
		-tags unit,integration,e2e \
		-failfast \
		-count=1 \
		./...

.PHONY: lint
lint:
	GOFLAGS="-tags=unit,integration,e2e,frontend" \
	go tool golangci-lint run --fix --new-from-rev HEAD --timeout=5m

.PHONY: lint-drone
lint-drone:
	GOFLAGS="-tags=unit,integration,e2e,frontend" \
	golangci-lint run --fix --timeout=5m


.PHONY: fmt
fmt:
	go fmt ./...
	GOFLAGS="-tags=unit,integration,e2e,frontend" \
	go vet ./...

.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux \
	go build -v \
		-trimpath \
		-ldflags "-w -s -X 'main.version=$(APP_VERSION)'" \
		-o $(OUTPUTDIR) ./...

.PHONY: build-frontend
build-frontend:
	VITE_API_URL="" GOFLAGS="-tags=frontend" go generate ./...
	APP_VERSION="$(APP_VERSION) (frontend)" GOFLAGS="-tags=frontend" $(MAKE) build
