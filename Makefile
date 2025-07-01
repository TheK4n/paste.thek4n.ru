APP_VERSION ?= not-set
OUTPUTDIR ?= bin/
COVER_PROFILE := $(shell mktemp)

.PHONY: e2e cover e2e-short integration build

default: build

e2e:
	GOMAXPROCS=1 \
	go test \
		-tags e2e \
		-failfast \
		-count=1 \
		./...

e2e-short:
	GOMAXPROCS=1 \
	go test \
		-tags e2e \
		-short \
		-failfast \
		-count=1 \
		./...

cover:
	GOMAXPROCS=1 \
	go test \
		-tags integration,e2e \
		-failfast \
		-count=1 \
		-cover -covermode=atomic \
		-coverprofile=$(COVER_PROFILE) \
		./...
	go tool cover -func=$(COVER_PROFILE)
	rm $(COVER_PROFILE)

integration:
	GOMAXPROCS=1 \
	go test \
		-tags integration \
		-failfast \
		-count=1 \
		./...

test:
	GOMAXPROCS=1 \
	go test \
		-tags integration,e2e \
		-failfast \
		-count=1 \
		./...

build:
	CGO_ENABLED=0 GOOS=linux \
	go build -v \
		-ldflags "-w -s -X 'main.version=$(APP_VERSION)'" \
		-o $(OUTPUTDIR) ./...
