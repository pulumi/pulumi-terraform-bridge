PROJECT          := github.com/pulumi/pulumi-terraform-bridge
TESTPARALLELISM  := 10

build::
	go mod tidy
	go build ${PROJECT}/v3/pkg/...
	go build ${PROJECT}/v3/internal/...
	go install ${PROJECT}/v3/cmd/...

fmt::
	@gofmt -w -s .

lint::
	golangci-lint run

test::
	go test -v -count=1 -cover -timeout 2h -parallel ${TESTPARALLELISM} ./...

# Run tests while accepting current output as expected output "golden"
# tests. In case where system behavior changes intentionally this can
# be useful to run to review the differences with git diff.
test_accept::
	PULUMI_ACCEPT=1 go test -v -count=1 -cover -timeout 2h -parallel ${TESTPARALLELISM} ./...
