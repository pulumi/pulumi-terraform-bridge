SHELL            := sh
PROJECT          := github.com/pulumi/pulumi-terraform-bridge
TESTPARALLELISM  := 10
export PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION := true

PROJECT_DIR := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))

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

RUN_TEST_CMD ?= ./...
GO_TEST_CMD ?= go test
test::
	@mkdir -p bin
	mise exec -- go build -o bin ./internal/testing/pulumi-terraform-bridge-test-provider
	PULUMI_TERRAFORM_BRIDGE_TEST_PROVIDER=$(PROJECT_DIR)/bin/pulumi-terraform-bridge-test-provider \
		mise exec -- $(value GO_TEST_CMD) -count=1 -coverprofile="coverage.txt" -coverpkg=./... -timeout 2h -parallel ${TESTPARALLELISM} $(value RUN_TEST_CMD)

# Run tests while accepting current output as expected output "golden"
# tests. In case where system behavior changes intentionally this can
# be useful to run to review the differences with git diff.
#
# Normal tests and golden acceptance share the mise-managed plugin pins.
# The mise postinstall hook sets up the local plugin home before tests start.
test_accept::
	PULUMI_ACCEPT=1 $(MAKE) test

.PHONY: test_plugin_isolation
test_plugin_isolation:
	python3 scripts/test_plugin_isolation.py

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
