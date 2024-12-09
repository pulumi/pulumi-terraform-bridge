preview:: export AWS_PROFILE := devsandbox
preview:: npm-add
	pulumi preview

npm-add:: packages-add
	npm add vpcaws@file:sdks/vpcaws

packages-add:: export PATH := $(WORKING_DIR)/bin:$(PATH)
packages-add:: bin/pulumi-resource-hcl
	rm -rf sdks
	pulumi package add ./bin/pulumi-resource-hcl terraform-aws-modules/vpc/aws 5.16.0
	# replace version placeholder with an actual version
	(cd sdks/vpcaws && jq '.version = "1.0.0"' package.json > package-tmp.json)
	cp sdks/vpcaws/package-tmp.json sdks/vpcaws/package.json
	rm sdks/vpcaws/package-tmp.json

bin/pulumi-resource-hcl::
	mkdir -p bin
	go build -o bin/pulumi-resource-hcl ../../cmd/pulumi-hcl-provider

.PHONY: compile npm-add packages-add bin/pulumi-resource-hcl
