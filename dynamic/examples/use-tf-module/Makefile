WORKING_DIR := $(shell pwd)

preview:: export PATH := $(WORKING_DIR)/bin:$(PATH)
preview:: export AWS_PROFILE := devsandbox
preview:: npm-add
	pulumi stack select dev
	(pulumi preview --diff >preview.log && echo OK)

npm-add:: packages-add
	npm add hcl@file:sdks/hcl

packages-add:: bin/pulumi-resource-hcl
	rm -rf sdks
	pulumi package add ./bin/pulumi-resource-hcl terraform-aws-modules/vpc/aws 5.16.0
	# replace version placeholder with an actual version
	(cd sdks/hcl && jq '.version = "1.0.0"' package.json > package-tmp.json)
	cp sdks/hcl/package-tmp.json sdks/hcl/package.json
	rm sdks/hcl/package-tmp.json


bin/pulumi-resource-hcl:: export LDFLAGS := -ldflags "-X google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=ignore"
bin/pulumi-resource-hcl::
	mkdir -p binx
	go build $(LDFLAGS) -o bin/pulumi-resource-hcl ../../cmd/pulumi-hcl-provider

.PHONY: compile npm-add packages-add bin/pulumi-resource-hcl
