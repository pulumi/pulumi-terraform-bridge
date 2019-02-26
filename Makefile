PROJECT_NAME := Pulumi Terraform Bridge
include build/common.mk

PROJECT         = github.com/pulumi/pulumi-terraform
GOPKGS          = $(shell go list ./pkg/... | grep -v /vendor/)
TESTPARALLELISM = 10

build::
	go build ${PROJECT}/pkg/tfgen
	go build ${PROJECT}/pkg/tfbridge

lint::
	golangci-lint run

test_fast::
	$(GO_TEST_FAST) ${GOPKGS}

test_all::
	$(GO_TEST) ${GOPKGS}

# The travis_* targets are entrypoints for CI.
.PHONY: travis_cron travis_push travis_pull_request travis_api
travis_cron: all
travis_push: all
travis_pull_request: all
travis_api: all
