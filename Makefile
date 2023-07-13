SHELL            := sh
PROJECT          := github.com/pulumi/pulumi-terraform-bridge
TESTPARALLELISM  := 10

PROJECT_DIR := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))

build::
	go mod tidy
	go build ${PROJECT}/v3/pkg/...
	go build ${PROJECT}/v3/internal/...
	cd testing && go build ./...

fmt::
	@gofmt -w -s .

lint::
	go run scripts/build.go lint

test::
	@mkdir -p bin
	go build -o bin ./internal/testing/pulumi-terraform-bridge-test-provider
	PULUMI_TERRAFORM_BRIDGE_TEST_PROVIDER=$(shell pwd)/bin/pulumi-terraform-bridge-test-provider \
		go test -v -count=1 -cover -timeout 2h -parallel ${TESTPARALLELISM} ./...
	cd testing && go test -v -count=1 ./...
	cd pkg/tests && go test -v -count=1 -cover -timeout 2h -parallel ${TESTPARALLELISM} ./...

	@echo Unit and integration tests for the muxer.
	cd x/muxer && go test -v -count=1 ./...
	cd x/muxer/tests && go test -v -count=1 ./...

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
	cd pf && go mod tidy
	cd pf/tests && go mod tidy
	cd pf/tests/internal/randomshim && go mod tidy
	cd pf/tests/internal/tlsshim && go mod tidy
	cd pf/tests/testdatagen/genrandom && go mod tidy
	cd pkg/tests && go mod tidy
	cd x/muxer && go mod tidy
	cd testing && go mod tidy
	go mod tidy
