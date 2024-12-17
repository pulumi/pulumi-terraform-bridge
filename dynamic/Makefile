build: bin
	go build \
		-o bin/pulumi-resource-terraform-provider \
		github.com/pulumi/pulumi-terraform-bridge/v3/dynamic

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

bin:
	mkdir bin

.PHONY: test
