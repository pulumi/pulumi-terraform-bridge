module github.com/pulumi/pulumi-terraform-bridge/v2

go 1.14

require (
	github.com/apparentlymart/go-cidr v1.0.1
	github.com/blang/semver v3.5.1+incompatible
	github.com/davecgh/go-spew v1.1.1
	github.com/gedex/inflector v0.0.0-20170307190818-16278e9db813
	github.com/go-test/deep v1.0.3
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.4.2
	github.com/hashicorp/errwrap v1.0.0
	github.com/hashicorp/go-cty v1.4.1-0.20200414143053-d3edf31b6320
	github.com/hashicorp/go-getter v1.4.2-0.20200106182914-9813cbd4eb02
	github.com/hashicorp/go-hclog v0.9.2
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/go-plugin v1.3.0
	github.com/hashicorp/go-uuid v1.0.1
	github.com/hashicorp/go-version v1.2.1
	github.com/hashicorp/hcl v1.0.0
	github.com/hashicorp/hcl/v2 v2.3.0
	github.com/hashicorp/hil v0.0.0-20190212132231-97b3a9cdfa93
	github.com/hashicorp/terraform-plugin-sdk v1.7.0
	github.com/hashicorp/terraform-plugin-sdk/v2 v2.0.2
	github.com/hashicorp/terraform-svchost v0.0.0-20191119180714-d2e4933b9136
	github.com/json-iterator/go v1.1.9
	github.com/mitchellh/cli v1.1.1
	github.com/mitchellh/copystructure v1.0.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/go-testing-interface v1.0.4
	github.com/mitchellh/hashstructure v1.0.0
	github.com/mitchellh/mapstructure v1.1.2
	github.com/mitchellh/reflectwalk v1.0.1
	github.com/pkg/errors v0.9.1
	github.com/pulumi/pulumi/pkg/v2 v2.11.3-0.20201009201355-249140242ebb
	github.com/pulumi/pulumi/sdk/v2 v2.11.3-0.20201009201355-249140242ebb
	github.com/spf13/afero v1.2.2
	github.com/spf13/cobra v1.0.0
	github.com/stretchr/testify v1.6.1
	github.com/terraform-providers/terraform-provider-archive v1.3.0
	github.com/terraform-providers/terraform-provider-http v1.2.0
	github.com/zclconf/go-cty v1.3.1
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/mod v0.3.0
	golang.org/x/net v0.0.0-20200707034311-ab3426394381
	google.golang.org/grpc v1.30.0
)

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.4.3+incompatible

replace github.com/hashicorp/terraform-plugin-sdk/v2 => github.com/pulumi/terraform-plugin-sdk/v2 v2.0.0-20200910230100-328eb4ff41df

replace github.com/pulumi/pulumi/sdk/v2 => ../pulumi/sdk

replace github.com/pulumi/pulumi/pkg/v2 => ../pulumi/pkg
