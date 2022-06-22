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
	go test -count=1 -cover -timeout 2h -parallel ${TESTPARALLELISM} ./...
