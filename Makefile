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
	golangci-lint run

test::
	@mkdir -p bin
	go build -o bin ./internal/testing/pulumi-terraform-bridge-test-provider
	PULUMI_TERRAFORM_BRIDGE_TEST_PROVIDER=$(shell pwd)/bin/pulumi-terraform-bridge-test-provider \
		go test -v -count=1 -cover -timeout 2h -parallel ${TESTPARALLELISM} ./...
	cd testing && go test -v -count=1 ./...
	cd pkg/tests && go test -v -count=1 -cover -timeout 2h -parallel ${TESTPARALLELISM} ./...

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
	cd testing && go mod tidy
	go mod tidy

# Update the subtree in pkg/tf2pulumi/internal/tf with the latest code from terraform master
TFPKGS=backend\|command\|provider-terraform\|provider-simple\|provider-simple-v6\|provisioner-local-exec\|cloud\|builtin\|grpcwrap\|initwd\|earlyconfig\|moduledeps\|plugin\|tfplugin5\|plugin6\|tfplugin6\|communicator\|providercache\|e2e\|legacy\|moduletest\|helper\|repl\|terminal
pull_terraform::
	git remote remove terraform || true
	git remote add -f -t main --no-tags terraform https://github.com/hashicorp/terraform.git
	git rm -rf ./pkg/tf2pulumi/internal/tf
	git read-tree --prefix=pkg/tf2pulumi/internal/tf/ -u terraform/main:internal
	# Remove the parts of Terraform we don't need
	find ./pkg/tf2pulumi/internal/tf -mindepth 1 -maxdepth 1 -type d -regex ".*/tf/\($(TFPKGS)\)" -exec rm -rf {} \;
	# No need for us to keep any of Terraforms tests
	find ./pkg/tf2pulumi/internal/tf -type f -regex ".*_test.go" -delete
	# Update the .go file import paths
	find ./pkg/tf2pulumi/internal/tf -type f -name "*.go" -exec sed -i -e 's|github.com/hashicorp/terraform/internal|github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/internal/tf|g' {} \;
	# Workaround the duplicate registration of rpcFriendlyDiag in tf/tfdiags/rpc_friendly.go and same type in the public plugin repo
	sed -i -e 's|rpcFriendlyDiag|internalRpcFriendlyDiag|g' ./pkg/tf2pulumi/internal/tf/tfdiags/rpc_friendly.go
	# Restore the README and LICENSE
	git restore --staged --worktree pkg/tf2pulumi/internal/tf/README.md pkg/tf2pulumi/internal/tf/LICENSE
	# Finally commit it
	git add ./pkg/tf2pulumi/internal/tf
	# Don't error if we can't commit it's probably just because there were no changes
	git commit -m "Updated terraform to $$(git show-ref -s terraform/main)" || true
	git remote remove terraform
