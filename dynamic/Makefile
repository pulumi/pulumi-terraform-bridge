build: bin/pulumi-resource-terraform-provider

_ := $(shell mkdir -p bin)

bin/pulumi-resource-terraform-provider:
	go build -o $@ github.com/pulumi/pulumi-terraform-bridge/v3/dynamic

install: build
	pulumi plugin rm resource terraform-provider --all --yes
	pulumi plugin install resource terraform-provider v0.0.0-dev \
		--file bin/pulumi-resource-terraform-provider --reinstall

test_unit:
	cd internal/shim && go test ./...
	go test -short ./...

test:
	cd internal/shim && go test -v ./...
	go test -v ./...

test_accept:
	go test -v ./... -update

.PHONY: bin/pulumi-resource-terraform-provider install build test_unit test test_acccept
