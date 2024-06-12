LDFLAGS := -ldflags "-X google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=ignore"

test.unit:
	cd internal/shim && go test ${LDFLAGS} ./...

test:
	cd internal/shim && go test ${LDFLAGS} ./...
