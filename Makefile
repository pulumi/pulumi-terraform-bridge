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
		go test -v -count=1 -coverprofile="coverage.txt" -coverpkg=./... -timeout 2h -parallel ${TESTPARALLELISM} ./...
	cd testing && go test -v -count=1 -coverprofile="coverage.txt" -coverpkg=./... ./...
	cd pkg/tests && go test -v -count=1 -coverprofile="coverage.txt" -coverpkg=./... -timeout 2h -parallel ${TESTPARALLELISM} ./...

	# Unit and integration tests for the muxer.
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
	find . -name go.mod -execdir go mod tidy \;

# Ideally, we would have `tidy: pin_upstream_sdk`, but `find` doesn't have the same format
# on windows.
pin_upstream_sdk: UpstreamPluginSDK=github.com/pulumi/terraform-plugin-sdk/v2
pin_upstream_sdk: OutPluginSDK=github.com/pulumi/terraform-plugin-sdk/v2
pin_upstream_sdk: PluginSDKVersion=v2.0.0-20230912190043-e6d96b3b8f7e
pin_upstream_sdk:
	# /x/muxer doesn't depend on the rest of the bridge or any TF libraries, so it
	# doesn't need this replace.
	#
	# All other modules depend on the bridge, so need this replace.
	find . -name go.mod -and \
	-not -path '*/x/muxer/*' -and \
	-execdir go mod edit -replace ${UpstreamPluginSDK}=${OurPluginSDK}@${PluginSDKVersion} \;
