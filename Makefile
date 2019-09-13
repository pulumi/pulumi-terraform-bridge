PROJECT_NAME := Pulumi Terraform Bridge
include build/common.mk

PACK             := terraform
PACKDIR          := sdk
NODE_MODULE_NAME := @pulumi/terraform
PROJECT          := github.com/pulumi/pulumi-terraform
GOPKGS           := $(shell go list ./pkg/... | grep -v /vendor/)
TESTPARALLELISM  := 10

VERSION          ?= $(shell scripts/get-version)
PYPI_VERSION     := $(shell scripts/get-py-version)

VERSION_FLAGS    := -ldflags "-X github.com/pulumi/pulumi-terraform/pkg/version.Version=${VERSION}"

build::
	go build ${PROJECT}/pkg/tfgen
	go build ${PROJECT}/pkg/tfbridge
	go install $(VERSION_FLAGS) ${PROJECT}/cmd/pulumi-resource-terraform
	cd ${PACKDIR}/nodejs/ && \
		yarn install && \
		yarn run tsc
	cp LICENSE ${PACKDIR}/nodejs/package.json ${PACKDIR}/nodejs/yarn.lock \
		${PACKDIR}/nodejs/bin
	cp README.package.md ${PACKDIR}/nodejs/bin/README.md
	sed -i.bak 's/$${VERSION}/$(VERSION)/g' ${PACKDIR}/nodejs/bin/package.json
	cd ${PACKDIR}/python/ && \
			if [ $$(command -v pandoc) ]; then \
				pandoc --from=markdown --to=rst --output=README.rst ../../README.md; \
			else \
				echo "warning: pandoc not found, not generating README.rst"; \
				echo "" > README.rst; \
			fi && \
			$(PYTHON) setup.py clean --all 2>/dev/null && \
			rm -rf ./bin/ ../python.bin/ && cp -R . ../python.bin && mv ../python.bin ./bin && \
			sed -i.bak -e "s/\$${VERSION}/$(PYPI_VERSION)/g" -e "s/\$${PLUGIN_VERSION}/$(VERSION)/g" ./bin/setup.py && \
			cd ./bin && $(PYTHON) setup.py build sdist


lint::
	golangci-lint run

install::
	go install $(VERSION_FLAGS) $(PROJECT)/cmd/pulumi-resource-terraform
	[ ! -e "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)" ] || rm -rf "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)"
	mkdir -p "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)"
	cp -r sdk/nodejs/bin/. "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)"
	rm -rf "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)/node_modules"
	rm -rf "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)/tests"
	cd "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)" && \
		yarn install --offline --production && \
		(yarn unlink > /dev/null 2>&1 || true) && \
		yarn link

test_fast:: install
	$(GO_TEST_FAST) ${GOPKGS} ./examples

test_all:: install
	$(GO_TEST) ${GOPKGS} ./examples

.PHONY: publish_tgz
publish_tgz:
	$(call STEP_MESSAGE)
	./scripts/publish_tgz.sh

.PHONY: check_clean_worktree
check_clean_worktree:
	$$(go env GOPATH)/src/github.com/pulumi/scripts/ci/check-worktree-is-clean.sh

# While pulumi-terraform is not built using tfgen/tfbridge, the layout of the source tree is the same as these
# style of repositories, so we can re-use the common publishing scripts.
.PHONY: publish_packages
publish_packages:
	$(call STEP_MESSAGE)
	$$(go env GOPATH)/src/github.com/pulumi/scripts/ci/publish-tfgen-package .
	$$(go env GOPATH)/src/github.com/pulumi/scripts/ci/build-package-docs.sh terraform

# The travis_* targets are entrypoints for CI.
.PHONY: travis_cron travis_push travis_pull_request travis_api
travis_cron: all
travis_push: all check_clean_worktree only_test publish_tgz publish_packages
travis_pull_request: all
travis_api: all
