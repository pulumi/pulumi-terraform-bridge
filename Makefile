SHELL            := sh
PROJECT          := github.com/pulumi/pulumi-terraform-bridge
TESTPARALLELISM  := 10
export PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION := true

PROJECT_DIR := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))

install_plugins::
	pulumi plugin install converter terraform 1.0.19
	pulumi plugin install resource random 4.16.3
	pulumi plugin install resource aws 6.22.2
	pulumi plugin install resource archive 0.0.4
	pulumi plugin install resource wavefront 3.0.0
	pulumi plugin install resource equinix 0.6.0 --server github://api.github.com/equinix

build::
	go mod tidy
	go build ${PROJECT}/v3/pkg/...
	go build ${PROJECT}/v3/internal/...

fmt::
	@gofmt -w -s .

lint:
	go run scripts/build.go lint

lint_fix:
	go run scripts/build.go fix-lint

test:: install_plugins
	@mkdir -p bin
	go build -o bin ./internal/testing/pulumi-terraform-bridge-test-provider
	PULUMI_TERRAFORM_BRIDGE_TEST_PROVIDER=$(shell pwd)/bin/pulumi-terraform-bridge-test-provider \
		go test -count=1 -coverprofile="coverage.txt" -coverpkg=./... -timeout 2h -parallel ${TESTPARALLELISM} ./...

# Run tests while accepting current output as expected output "golden"
# tests. In case where system behavior changes intentionally this can
# be useful to run to review the differences with git diff.
test_accept::
	PULUMI_ACCEPT=1 go test -v -count=1 -cover -timeout 2h -parallel ${TESTPARALLELISM} ./...

generate_builtins_test::
	if [ ! -d ./scripts/venv ]; then python -m venv ./scripts/venv; fi
	. ./scripts/venv/*/activate && python -m pip install -r ./scripts/requirements.txt
	. ./scripts/venv/*/activate &&  python ./scripts/generate_builtins.py


tidy::
	find . -name go.mod -execdir go mod tidy \;

# Ideally, we would have `tidy: pin_upstream_sdk`, but `find` doesn't have the same format
# on windows.
pin_upstream_sdk: UpstreamPluginSDK=github.com/hashicorp/terraform-plugin-sdk/v2
pin_upstream_sdk: OurPluginSDK=github.com/pulumi/terraform-plugin-sdk/v2
pin_upstream_sdk: PluginSDKVersion=v2.0.0-20240520223432-0c0bf0d65f10
pin_upstream_sdk:
	# /x/muxer doesn't depend on the rest of the bridge or any TF libraries, so it
	# doesn't need this replace.
	#
	# All other modules depend on the bridge, so need this replace.
	find . -name go.mod -and \
	-not -path '*/x/muxer/*' -and \
	-execdir go mod edit -replace ${UpstreamPluginSDK}=${OurPluginSDK}@${PluginSDKVersion} \;

.PHONY: go.work
go.work::
	@cd $(PROJECT_DIR)
ifeq (,$(wildcard $(PROJECT_DIR)/go.work))
	@echo "Initializing go.work..."
	@go work init
else
	@echo "Updating go.work..."
endif
	@go work use -r .
