SHELL            := sh
PROJECT          := github.com/pulumi/pulumi-terraform-bridge
TESTPARALLELISM  := 10
export PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION := true

PROJECT_DIR := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))

build::
	go mod tidy
	go build ${PROJECT}/v3/pkg/...
	go build ${PROJECT}/v3/internal/...

install_plugins::
	pulumi plugin install converter terraform 1.0.20
	pulumi plugin install resource random 4.16.3
	pulumi plugin install resource aws 6.22.2
	pulumi plugin install resource archive 0.0.4
	pulumi plugin install resource wavefront 3.0.0
	pulumi plugin install resource auth0 3.16.0
	pulumi plugin install resource http 0.0.11
	pulumi plugin install resource gcp 8.22.0
	pulumi plugin install resource equinix 0.6.0 --server github://api.github.com/equinix

fmt::
	@gofmt -w -s .

lint:
	go run scripts/build.go lint

lint_fix:
	go run scripts/build.go fix-lint

RUN_TEST_CMD ?= ./...
GO_TEST_CMD ?= go test
test:: install_plugins
	@mkdir -p bin
	go build -o bin ./internal/testing/pulumi-terraform-bridge-test-provider
	PULUMI_TERRAFORM_BRIDGE_TEST_PROVIDER=$(shell pwd)/bin/pulumi-terraform-bridge-test-provider \
		$(value GO_TEST_CMD) -count=1 -coverprofile="coverage.txt" -coverpkg=./... -timeout 2h -parallel ${TESTPARALLELISM} $(value RUN_TEST_CMD)

# Run tests while accepting current output as expected output "golden"
# tests. In case where system behavior changes intentionally this can
# be useful to run to review the differences with git diff.
#
# For re-recording golden files, we use a local, test-specific plugin
# cache to ensure parity with CI.
TEST_PULUMI_HOME := $(PROJECT_DIR)/.pulumi-test
test_accept::
	rm -rf $(TEST_PULUMI_HOME)
	PULUMI_HOME=$(TEST_PULUMI_HOME) $(MAKE) install_plugins
	PULUMI_HOME=$(TEST_PULUMI_HOME) PULUMI_ACCEPT=1 go test -v -count=1 -cover -timeout 2h -parallel ${TESTPARALLELISM} $(value RUN_TEST_CMD)

generate_builtins_test::
	if [ ! -d ./scripts/venv ]; then python -m venv ./scripts/venv; fi
	. ./scripts/venv/*/activate && python -m pip install -r ./scripts/requirements.txt
	. ./scripts/venv/*/activate &&  python ./scripts/generate_builtins.py


tidy::
	find . -name go.mod -execdir go mod tidy \;

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
