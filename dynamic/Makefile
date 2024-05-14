LDFLAGS := -ldflags "-X google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=ignore"

build: bin
	go build \
		-o bin/pulumi-resource-terraform-bridge \
		${LDFLAGS} \
		github.com/pulumi/pulumi-terraform-bridge/dynamic

test.unit:
	cd internal/shim && go test ${LDFLAGS} ./...
	go test -short ${LDFLAGS} ./...

test:
	cd internal/shim && go test ${LDFLAGS} ./...
	go test ${LDFLAGS} ./... -count 1

test.update:
	go test ${LDFLAGS} ./... -update

bin:
	mkdir bin
