build.tests::
	go test -test.run NONE
	cd tests && go test -test.run NONE
	cd tests/integration && go test -test.run NONE

test::
	go test
	cd tests && go test

test.integration::
	cd tests/integration && go test -test.v

test.cover::
	cd tests && go test -cover -coverpkg=github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge -coverprofile=cover.out
	go tool cover -html=tests/cover.out

build::
	go build -v all
	cd tests && go build -v all
	cd tests/integration && go build -v all

build.testproviders:: \
	tests/bin/pulumi-resource-testbridge \
	tests/bin/pulumi-resource-random

tests/bin/pulumi-resource-random:: tests/bin/pulumi-tfgen-random
	./tests/bin/pulumi-tfgen-random schema --out tests/internal/cmd/pulumi-resource-random/
	rm tests/internal/cmd/pulumi-resource-random/Pulumi.yaml
	(cd tests/internal/cmd/pulumi-resource-random && go build -o ../../../bin/)

tests/bin/pulumi-tfgen-random::
	mkdir -p tests/bin
	(cd tests/internal/cmd/pulumi-tfgen-random && go build -o ../../../bin/)

tests/bin/pulumi-resource-testbridge:: tests/bin/pulumi-tfgen-testbridge
	./tests/bin/pulumi-tfgen-testbridge schema --out tests/internal/cmd/pulumi-resource-testbridge/
	rm tests/internal/cmd/pulumi-resource-testbridge/Pulumi.yaml
	(cd tests/internal/cmd/pulumi-resource-testbridge && go build -o ../../../bin/)

tests/bin/pulumi-tfgen-testbridge::
	mkdir -p tests/bin
	(cd tests/internal/cmd/pulumi-tfgen-testbridge && go build -o ../../../bin/)

gen.testdata::
	(cd tests/testdatagen/genrandom && ./generate.sh)

.PHONY: Test
