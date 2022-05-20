module github.com/pulumi/pulumi-terraform-bridge/v3

go 1.16

require (
	github.com/apparentlymart/go-cidr v1.1.0
	github.com/blang/semver v3.5.1+incompatible
	github.com/davecgh/go-spew v1.1.1
	github.com/gedex/inflector v0.0.0-20170307190818-16278e9db813
	github.com/go-test/deep v1.0.3
	github.com/golang/glog v1.0.0
	github.com/golang/protobuf v1.5.2
	github.com/hashicorp/errwrap v1.1.0
	github.com/hashicorp/go-cty v1.4.1-0.20200414143053-d3edf31b6320
	github.com/hashicorp/go-getter v1.5.3
	github.com/hashicorp/go-hclog v1.2.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/go-plugin v1.4.3
	github.com/hashicorp/go-uuid v1.0.3
	github.com/hashicorp/go-version v1.4.0
	github.com/hashicorp/hcl v1.0.0
	github.com/hashicorp/hcl/v2 v2.12.0
	github.com/hashicorp/hil v0.0.0-20190212132231-97b3a9cdfa93
	github.com/hashicorp/terraform-plugin-sdk v1.7.0
	github.com/hashicorp/terraform-plugin-sdk/v2 v2.13.0
	github.com/hashicorp/terraform-svchost v0.0.0-20200729002733-f050f53b9734
	github.com/json-iterator/go v1.1.12
	github.com/mitchellh/cli v1.1.2
	github.com/mitchellh/copystructure v1.2.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/go-testing-interface v1.14.1
	github.com/mitchellh/hashstructure v1.0.0
	github.com/mitchellh/mapstructure v1.5.0
	github.com/mitchellh/reflectwalk v1.0.2
	github.com/pkg/errors v0.9.1
	github.com/pulumi/pulumi-java/pkg v0.2.1
	github.com/pulumi/pulumi-yaml v0.4.0
	github.com/pulumi/pulumi/pkg/v3 v3.33.1
	github.com/pulumi/pulumi/sdk/v3 v3.33.1
	github.com/pulumi/terraform-diff-reader v0.0.0-20201211191010-ad4715e9285e
	github.com/russross/blackfriday/v2 v2.1.0
	github.com/spf13/afero v1.6.0
	github.com/spf13/cobra v1.4.0
	github.com/stretchr/testify v1.7.1
	github.com/terraform-providers/terraform-provider-archive v1.3.0
	github.com/terraform-providers/terraform-provider-http v1.2.0
	github.com/zclconf/go-cty v1.10.0
	golang.org/x/crypto v0.0.0-20220131195533-30dcbda58838
	golang.org/x/mod v0.5.0
	golang.org/x/net v0.0.0-20220325170049-de3da57026de
	google.golang.org/grpc v1.45.0
	google.golang.org/protobuf v1.28.0
)

replace (
	github.com/hashicorp/terraform-plugin-sdk/v2 => github.com/pulumi/terraform-plugin-sdk/v2 v2.0.0-20220516192341-2de4e601e694
	github.com/pulumi/pulumi-yaml => github.com/pulumi/pulumi-yaml v0.4.0
)
