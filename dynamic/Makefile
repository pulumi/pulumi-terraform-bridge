LDFLAGS := -ldflags "-X google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=ignore"

build: bin
	go build \
		-o bin/pulumi-resource-terraform-provider \
		${LDFLAGS} \
		github.com/pulumi/pulumi-terraform-bridge/dynamic

test.unit:
	cd internal/shim && go test ${LDFLAGS} ./...
	go test -short ${LDFLAGS} ./...

test:
	cd internal/shim && go test -v ${LDFLAGS} ./...
	go test -v ${LDFLAGS} ./...

test.update:
	go test -v ${LDFLAGS} ./... -update

bin:
	mkdir bin

.PHONY: test
