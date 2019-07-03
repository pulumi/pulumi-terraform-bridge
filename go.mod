module github.com/pulumi/pulumi-terraform

go 1.12

require (
	github.com/apache/thrift v0.12.0 // indirect
	github.com/docker/docker v1.13.1 // indirect
	github.com/gedex/inflector v0.0.0-20170307190818-16278e9db813
	github.com/gliderlabs/ssh v0.1.3 // indirect
	github.com/go-ini/ini v1.31.0 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.3.1
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/hcl v0.0.0-20170504190234-a4b07c25de5f
	github.com/hashicorp/terraform v0.12.0-rc1.0.20190509225429-28b2383eacae
	github.com/mitchellh/copystructure v1.0.0
	github.com/openzipkin/zipkin-go v0.1.6 // indirect
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v0.9.3-0.20190127221311-3c4408c8b829 // indirect
	github.com/pulumi/pulumi v0.17.22-0.20190702185104-ebceea93a5da
	github.com/smartystreets/goconvey v0.0.0-20190330032615-68dc04aab96a // indirect
	github.com/spf13/cobra v0.0.3
	github.com/stretchr/testify v1.3.1-0.20190311161405-34c6fa2dc709
	github.com/zclconf/go-cty v0.0.0-20190430221426-d36a6f0dbffd
	golang.org/x/build v0.0.0-20190314133821-5284462c4bec // indirect
	golang.org/x/net v0.0.0-20190502183928-7f726cade0ab
	google.golang.org/grpc v1.20.1
	gopkg.in/ini.v1 v1.42.0 // indirect
	gopkg.in/vmihailenco/msgpack.v2 v2.9.1 // indirect
)

replace (
	github.com/Nvveen/Gotty => github.com/ijc25/Gotty v0.0.0-20170406111628-a8b993ba6abd
	github.com/golang/glog => github.com/pulumi/glog v0.0.0-20180820174630-7eaa6ffb71e4
)
