---
# *** WARNING: This file was auto-generated. Do not edit by hand unless you're certain you know what you are doing! ***
title: Libvirt Provider
meta_desc: Provides an overview on how to configure the Pulumi Libvirt provider.
layout: package
---
## Installation

The Libvirt provider is available as a package in all Pulumi languages:

* JavaScript/TypeScript: [`@pulumi/libvirt`](https://www.npmjs.com/package/@pulumi/libvirt)
* Python: [`pulumi-libvirt`](https://pypi.org/project/pulumi-libvirt/)
* Go: [`github.com/pulumi/pulumi-libvirt/sdk/go/libvirt`](https://github.com/pulumi/pulumi-libvirt)
* .NET: [`Pulumi.Libvirt`](https://www.nuget.org/packages/Pulumi.Libvirt)
* Java: [`com.pulumi/libvirt`](https://central.sonatype.com/artifact/com.pulumi/libvirt)
## Overview

The Libvirt provider is used to interact with Linux
[libvirt](https://libvirt.org) hypervisors.

The provider needs to be configured with the proper connection information
before it can be used.

> **Note:** while libvirt can be used with several types of hypervisors, this
provider focuses on [KVM](http://libvirt.org/drvqemu.html). Other drivers may not be
working and haven't been tested.
## The connection URI

The provider understands [connection URIs](https://libvirt.org/uri.html). The supported transports are:

* `tcp` (non-encrypted connection)
* `unix` (UNIX domain socket)
* `tls` (See [here](https://libvirt.org/kbase/tlscerts.html) for information how to setup certificates)
* `ssh` (Secure shell)

Unlike the original libvirt, the `ssh` transport is not implemented using the ssh command and therefore does not require `nc` (netcat) on the server side.

Additionally, the `ssh` URI supports passwords using the `driver+ssh://[username:PASSWORD@][hostname][:port]/[path]?sshauth=ssh-password` syntax.

As the provider does not use libvirt on the client side, not all connection URI options are supported or apply.
## Example Usage

{{< chooser language "typescript,python,go,csharp,java,yaml" >}}
{{% choosable language typescript %}}
```yaml
# Pulumi.yaml provider configuration file
name: configuration-example
runtime: nodejs
config:
    simple-provider:authUrl:
        value: http://myauthurl:5000/v3
    simple-provider:password:
        value: pwd
    simple-provider:region:
        value: RegionOne
    simple-provider:tenantName:
        value: admin
    simple-provider:userName:
        value: admin

```
```typescript
import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

//# Define a resource
const aResource = new simple.index.Resource("a_resource", {
    inputOne: "hello",
    inputTwo: true,
});
```
{{% /choosable %}}
{{% choosable language python %}}
```yaml
# Pulumi.yaml provider configuration file
name: configuration-example
runtime: python
config:
    simple-provider:authUrl:
        value: http://myauthurl:5000/v3
    simple-provider:password:
        value: pwd
    simple-provider:region:
        value: RegionOne
    simple-provider:tenantName:
        value: admin
    simple-provider:userName:
        value: admin

```
```python
import pulumi
import pulumi_simple as simple

## Define a resource
a_resource = simple.index.Resource("a_resource",
    input_one=hello,
    input_two=True)
```
{{% /choosable %}}
{{% choosable language csharp %}}
```yaml
# Pulumi.yaml provider configuration file
name: configuration-example
runtime: dotnet
config:
    simple-provider:authUrl:
        value: http://myauthurl:5000/v3
    simple-provider:password:
        value: pwd
    simple-provider:region:
        value: RegionOne
    simple-provider:tenantName:
        value: admin
    simple-provider:userName:
        value: admin

```
```csharp
using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Simple = Pulumi.Simple;

return await Deployment.RunAsync(() =>
{
    //# Define a resource
    var aResource = new Simple.Index.Resource("a_resource", new()
    {
        InputOne = "hello",
        InputTwo = true,
    });

});

```
{{% /choosable %}}
{{% choosable language go %}}
```yaml
# Pulumi.yaml provider configuration file
name: configuration-example
runtime: go
config:
    simple-provider:authUrl:
        value: http://myauthurl:5000/v3
    simple-provider:password:
        value: pwd
    simple-provider:region:
        value: RegionOne
    simple-provider:tenantName:
        value: admin
    simple-provider:userName:
        value: admin

```
```go
package main

import (
	"github.com/pulumi/pulumi-simple/sdk/go/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// # Define a resource
		_, err := simple.NewResource(ctx, "a_resource", &simple.ResourceArgs{
			InputOne: "hello",
			InputTwo: true,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
```
{{% /choosable %}}
{{% choosable language yaml %}}
```yaml
# Pulumi.yaml provider configuration file
name: configuration-example
runtime: yaml
config:
    simple-provider:authUrl:
        value: http://myauthurl:5000/v3
    simple-provider:password:
        value: pwd
    simple-provider:region:
        value: RegionOne
    simple-provider:tenantName:
        value: admin
    simple-provider:userName:
        value: admin

```
```yaml
resources:
  ## Define a resource
  aResource:
    type: simple:resource
    name: a_resource
    properties:
      inputOne: hello
      inputTwo: true
```
{{% /choosable %}}
{{% choosable language java %}}
```yaml
# Pulumi.yaml provider configuration file
name: configuration-example
runtime: java
config:
    simple-provider:authUrl:
        value: http://myauthurl:5000/v3
    simple-provider:password:
        value: pwd
    simple-provider:region:
        value: RegionOne
    simple-provider:tenantName:
        value: admin
    simple-provider:userName:
        value: admin

```
```java
package generated_program;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.core.Output;
import com.pulumi.simple.resource;
import com.pulumi.simple.ResourceArgs;
import java.util.List;
import java.util.ArrayList;
import java.util.Map;
import java.io.File;
import java.nio.file.Files;
import java.nio.file.Paths;

public class App {
    public static void main(String[] args) {
        Pulumi.run(App::stack);
    }

    public static void stack(Context ctx) {
        //# Define a resource
        var aResource = new Resource("aResource", ResourceArgs.builder()
            .inputOne("hello")
            .inputTwo(true)
            .build());

    }
}
```
{{% /choosable %}}
{{< /chooser >}}
## Configuration Reference

The following keys can be used to configure the provider.

* `uri` - (Required) The [connection URI](https://libvirt.org/uri.html) used
  to connect to the libvirt host.
## Environment variables

The libvirt connection URI can also be specified with the `LIBVIRT_DEFAULT_URI`
shell environment variable.

```
$ export LIBVIRT_DEFAULT_URI="qemu+ssh://root@192.168.1.100/system"
$ pulumi preview
```