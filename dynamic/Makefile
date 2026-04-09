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

# For re-recording golden files, we use a local, test-specific plugin
# cache to ensure parity with CI.
TEST_PULUMI_HOME := $(abspath ../.pulumi-test)
test_accept:
	rm -rf $(TEST_PULUMI_HOME)
	PULUMI_HOME=$(TEST_PULUMI_HOME) $(MAKE) -C .. install_plugins
	PULUMI_HOME=$(TEST_PULUMI_HOME) PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION=true \
		go test -v . ./parameterize -update

.PHONY: bin/pulumi-resource-terraform-provider install build test_unit test test_acccept
