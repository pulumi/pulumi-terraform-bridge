TESTPARALLELISM := 10

build.tests::
	go test -test.run NONE
	cd tests && go test -test.run NONE
	cd tests/integration && go test -test.run NONE

test::
	go test -v -count=1 -cover -timeout 2h -parallel ${TESTPARALLELISM} ./...
	cd tests && go test -v -count=1 -cover -timeout 2h -parallel ${TESTPARALLELISM} # tests have a separte go.mod


test.integration::
	cd tests/integration && go test -v -count=1 -timeout 2h -parallel ${TESTPARALLELISM}

test.cover::
	cd tests && go test -cover -coverpkg=github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge -coverprofile=cover.out
	go tool cover -html=tests/cover.out

build::
	go build -v all
	cd tests && go build -v all
	cd tests/integration && go build -v all

build.testproviders:: \
	tests/bin/pulumi-resource-testbridge \
	tests/bin/pulumi-resource-random \
	tests/bin/pulumi-resource-tls

tests/bin/pulumi-resource-random:: tests/bin/pulumi-tfgen-random
	./tests/bin/pulumi-tfgen-random schema --out tests/internal/cmd/pulumi-resource-random/
	rm tests/internal/cmd/pulumi-resource-random/Pulumi.yaml
	(cd tests/internal/cmd/pulumi-resource-random && go build -o ../../../bin/)

tests/bin/pulumi-tfgen-random::
	mkdir -p tests/bin
	(cd tests/internal/cmd/pulumi-tfgen-random && go build -o ../../../bin/)

tests/bin/pulumi-resource-tls:: tests/bin/pulumi-tfgen-tls
	./tests/bin/pulumi-tfgen-tls schema --out tests/internal/cmd/pulumi-resource-tls/
	rm tests/internal/cmd/pulumi-resource-tls/Pulumi.yaml
	(cd tests/internal/cmd/pulumi-resource-tls && go build -o ../../../bin/)

tests/bin/pulumi-tfgen-tls::
	mkdir -p tests/bin
	(cd tests/internal/cmd/pulumi-tfgen-tls && go build -o ../../../bin/)

tests/bin/pulumi-resource-testbridge:: tests/bin/pulumi-tfgen-testbridge
	./tests/bin/pulumi-tfgen-testbridge schema --out tests/internal/cmd/pulumi-resource-testbridge/
	rm tests/internal/cmd/pulumi-resource-testbridge/Pulumi.yaml
	(cd tests/internal/cmd/pulumi-resource-testbridge && go build -o ../../../bin/)

tests/bin/pulumi-tfgen-testbridge::
	mkdir -p tests/bin
	(cd tests/internal/cmd/pulumi-tfgen-testbridge && go build -o ../../../bin/)

gen.testdata::
	(cd tests/testdatagen/genrandom && ./generate.sh)

.PHONY: test tidy

tidy::
	go mod tidy
	cd tests && go mod tidy
	cd tests/internal/randomshim && go mod tidy
	cd tests/internal/tlsshim && go mod tidy
	cd tests/testdatagen/genrandom && go mod tidy

.PHONY: todos
