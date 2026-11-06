build: bin/pulumi-resource-terraform-provider

_ := $(shell mkdir -p bin)

bin/pulumi-resource-terraform-provider:
	go build -o $@ github.com/pulumi/pulumi-terraform-bridge/v3/dynamic

install: build
	mise exec -- pulumi plugin rm resource terraform-provider --all --yes
	mise exec -- pulumi plugin install resource terraform-provider v0.0.0-dev \
		--file bin/pulumi-resource-terraform-provider --reinstall

test_unit:
	cd internal/shim && mise exec -- go test ./...
	mise exec -- go test -short ./...

test:
	cd internal/shim && mise exec -- go test -v ./...
	mise exec -- go test -v ./...

# Use the same local plugin home and pins as normal tests.
RUN_TEST_CMD ?= . ./parameterize
test_accept:
	PULUMI_ACCEPT=1 mise exec -- go test -v -count=1 $(value RUN_TEST_CMD) -update

.PHONY: bin/pulumi-resource-terraform-provider install build test_unit test test_accept
