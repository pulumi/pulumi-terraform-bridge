PROJECT          := github.com/pulumi/pulumi-terraform-bridge
TESTPARALLELISM  := 10

build::
	go mod tidy
	go build ${PROJECT}/v3/pkg/...
	go build ${PROJECT}/v3/internal/...

fmt::
	@gofmt -w -s .

lint::
	golangci-lint run

test::
	@mkdir -p bin
	go build -o bin ./internal/testing/pulumi-terraform-bridge-test-provider
	PATH="${PWD}/bin:${PATH}" go test -v -count=1 -cover -timeout 2h -parallel ${TESTPARALLELISM} ./...

# Run tests while accepting current output as expected output "golden"
# tests. In case where system behavior changes intentionally this can
# be useful to run to review the differences with git diff.
test_accept::
	PULUMI_ACCEPT=1 go test -v -count=1 -cover -timeout 2h -parallel ${TESTPARALLELISM} ./...

generate_builtins_test::
	if [ ! -d ./scripts/venv ]; then python -m venv ./scripts/venv; fi
	. ./scripts/venv/*/activate && python -m pip install -r ./scripts/requirements.txt
	. ./scripts/venv/*/activate &&  python ./scripts/generate_builtins.py