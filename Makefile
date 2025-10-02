.PHONY: all build build-admin lint test test-e2e clean goreleaser-dev-build install-tools install-foundry run-op gendoc

# Go parameters
COMMIT_SHA = $(shell git rev-parse HEAD)
#TODO (DEVSVCS-2016) clean the GOLANG_PROTOBUF_REGISTRATION_CONFLICT flag
GOCMD=GOLANG_PROTOBUF_REGISTRATION_CONFLICT=warn go
GORELEASER=goreleaser
GOLANGCI_LINT=golangci-lint
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GORUN=$(GOCMD) run
#TODO (DEVSVCS-2016) clean the conflictPolicy=ignore flag
BUILD_FLAGS=-ldflags "-w -X 'github.com/smartcontractkit/cre-cli/cmd/version.Version=build $(COMMIT_SHA)' -X google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=ignore"
BINARY_NAME=cre
ADMIN_BINARY_NAME=cre-admin
GENDOC_BINARY_NAME=gendoc-cli

all: clean test build

build:
	$(GOBUILD) $(BUILD_FLAGS) -o $(BINARY_NAME) -v

lint:
	$(GOLANGCI_LINT) --color=always run ./... --fix -v

test: lint
	$(GOTEST) -v $$(go list ./... | grep -v usbwallet)

test-e2e:
	$(GOTEST) -v -p 5 ./test/

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(ADMIN_BINARY_NAME)
	rm -f $(GENDOC_BINARY_NAME)
	rm -rf dist
	rm -f test/proposal_*.json
	rm -f test/.proposal_*.json

run: build
	./$(BINARY_NAME) $(CMD)

GORELEASER_CONFIG ?= .goreleaser.yml

goreleaser-dev-build:
	$(GORELEASER) build --snapshot --clean --config=$(GORELEASER_CONFIG)

install-tools: install-foundry
	asdf plugin-add golang https://github.com/kennyp/asdf-golang.git
	asdf plugin add golangci-lint https://github.com/hypnoglow/asdf-golangci-lint.git
	asdf plugin-add goreleaser https://github.com/kforsthoevel/asdf-goreleaser.git
	asdf install

install-foundry:
	curl -L https://foundry.paradigm.xyz | bash
	foundryup --install v1.1.0

run-op:
	op run --env-file=".env" -- ./$(BINARY_NAME) $(CMD)

gendoc:
	rm -f docs/*
	$(GORUN) gendoc/main.go
