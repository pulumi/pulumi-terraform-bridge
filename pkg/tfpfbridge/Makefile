build.tests::
	go test -test.run NONE
	cd tests && go test -test.run NONE
	cd tests/integration && go test -test.run NONE

test::
	cd tests && go test -test.v

test.slow::
	cd tests/integration && go test -test.v

build::
	go build -v all
	cd tests && go build -v all
	cd tests/integration && go build -v all

.PHONY: Test
