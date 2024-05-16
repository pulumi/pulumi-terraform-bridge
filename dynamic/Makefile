build: bin
	go build \
		-o bin/pulumi-resource-terraform-bridge \
		-ldflags "-X google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=ignore" \
		github.com/pulumi/pulumi-terraform-bridge/dynamic

bin:
	mkdir bin
