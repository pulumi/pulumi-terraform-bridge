TESTPARALLELISM := 10
export PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION := true

install_plugins::
	pulumi plugin install resource random 4.16.3

build.tests:: install_plugins
	go test -test.run NONE
	cd tests && go test -test.run NONE
	cd tests/integration && go test -test.run NONE

test::	test.unit test.integration

test.unit::
	go test -v -short -count=1 -cover -coverpkg=./... -coverprofile ./coverage.out -timeout 2h -parallel ${TESTPARALLELISM} ./...

test.integration::
	cd tests/integration && go test -v -count=1 -timeout 2h -parallel ${TESTPARALLELISM}

test.cover::
	cd tests && go test -cover -coverpkg=github.com/pulumi/pulumi-terraform-bridge/v3/pf/tfbridge -coverprofile=cover.out
	go tool cover -html=tests/cover.out

build::
	go build -v all
	cd tests && go build -v all
	cd tests/integration && go build -v all

build.testproviders:: \
	tests/bin/pulumi-resource-testbridge \
	tests/bin/pulumi-resource-random \
	tests/bin/pulumi-resource-tls \
	tests/bin/pulumi-resource-muxedrandom

tests/bin/pulumi-resource-random:: tests/bin/pulumi-tfgen-random
	./tests/bin/pulumi-tfgen-random schema --out tests/internal/testprovider/cmd/pulumi-resource-random/
	rm tests/internal/testprovider/cmd/pulumi-resource-random/Pulumi.yaml
	(cd tests/internal/testprovider/cmd/pulumi-resource-random && go build -o ../../../../bin/)

tests/bin/pulumi-tfgen-random::
	mkdir -p tests/bin
	echo '{}' > tests/internal/testprovider/cmd/pulumi-resource-random/bridge-metadata.json
	(cd tests/internal/testprovider/cmd/pulumi-tfgen-random && go build -o ../../../../bin/)

tests/bin/pulumi-resource-tls:: tests/bin/pulumi-tfgen-tls
	./tests/bin/pulumi-tfgen-tls schema --out tests/internal/testprovider/cmd/pulumi-resource-tls/
	rm tests/internal/testprovider/cmd/pulumi-resource-tls/Pulumi.yaml
	(cd tests/internal/testprovider/cmd/pulumi-resource-tls && go build -o ../../../../bin/)

tests/bin/pulumi-tfgen-tls::
	mkdir -p tests/bin
	echo '{}' > tests/internal/testprovider/cmd/pulumi-resource-tls/bridge-metadata.json
	(cd tests/internal/testprovider/cmd/pulumi-tfgen-tls && go build -o ../../../../bin/)

tests/bin/pulumi-resource-testbridge:: tests/bin/pulumi-tfgen-testbridge
	./tests/bin/pulumi-tfgen-testbridge schema --out tests/internal/testprovider/cmd/pulumi-resource-testbridge/
	rm tests/internal/testprovider/cmd/pulumi-resource-testbridge/Pulumi.yaml
	(cd tests/internal/testprovider/cmd/pulumi-resource-testbridge && go build -o ../../../../bin/)

tests/bin/pulumi-tfgen-testbridge::
	mkdir -p tests/bin
	echo '{}' > tests/internal/testprovider/cmd/pulumi-resource-testbridge/bridge-metadata.json
	(cd tests/internal/testprovider/cmd/pulumi-tfgen-testbridge && go build -o ../../../../bin/)

tests/bin/pulumi-resource-muxedrandom:: tests/bin/pulumi-tfgen-muxedrandom
	./tests/bin/pulumi-tfgen-muxedrandom schema --out tests/internal/testprovider/cmd/pulumi-resource-muxedrandom/
	rm tests/internal/testprovider/cmd/pulumi-resource-muxedrandom/Pulumi.yaml
	(cd tests/internal/testprovider/cmd/pulumi-resource-muxedrandom && go build -o ../../../../bin/)

tests/bin/pulumi-tfgen-muxedrandom::
	mkdir -p tests/bin
	echo '{}' > tests/internal/testprovider/cmd/pulumi-resource-muxedrandom/bridge-metadata.json
	(cd tests/internal/testprovider/cmd/pulumi-tfgen-muxedrandom && go build -o ../../../../bin/)

.PHONY: test tidy

tidy::
	go mod tidy
	cd tests && go mod tidy
	cd dynamic && go mod tidy

.PHONY: todos
