This example will only translate the resource code. It has no configuration file.

## Example Usage

{{< chooser language "typescript,python,go,csharp,java,yaml" >}}
{{% choosable language typescript %}}
```typescript
import * as pulumi from "@pulumi/pulumi";
import * as openstack from "@pulumi/openstack";

//# Define a resource
const aResource = new openstack.index.Resource("a_resource", {
    renamedInput1: "hello",
    inputTwo: true,
});
export const someOutput = aResource.result;
```
{{% /choosable %}}
{{% choosable language python %}}
```python
import pulumi
import pulumi_openstack as openstack

## Define a resource
a_resource = openstack.index.Resource("a_resource",
    renamed_input1=hello,
    input_two=True)
pulumi.export("someOutput", a_resource["result"])
```
{{% /choosable %}}
{{% choosable language csharp %}}
```csharp
using System.Collections.Generic;
using System.Linq;
using Pulumi;
using OpenStack = Pulumi.OpenStack;

return await Deployment.RunAsync(() => 
{
    //# Define a resource
    var aResource = new OpenStack.Index.Resource("a_resource", new()
    {
        RenamedInput1 = "hello",
        InputTwo = true,
    });

    return new Dictionary<string, object?>
    {
        ["someOutput"] = aResource.Result,
    };
});

```
{{% /choosable %}}
{{% choosable language go %}}
```go
package main

import (
	"github.com/pulumi/pulumi-openstack/sdk/v5/go/openstack"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// # Define a resource
		aResource, err := openstack.NewResource(ctx, "a_resource", &openstack.ResourceArgs{
			RenamedInput1: "hello",
			InputTwo:      true,
		})
		if err != nil {
			return err
		}
		ctx.Export("someOutput", aResource.Result)
		return nil
	})
}
```
{{% /choosable %}}
{{% choosable language yaml %}}
```yaml
resources:
  ## Define a resource
  aResource:
    type: openstack:resource
    name: a_resource
    properties:
      renamedInput1: hello
      inputTwo: true
outputs:
  someOutput: ${aResource.result}
```
{{% /choosable %}}
{{% choosable language java %}}
```java
package generated_program;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.core.Output;
import com.pulumi.openstack.resource;
import com.pulumi.openstack.resourceArgs;
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
            .renamedInput1("hello")
            .inputTwo(true)
            .build());

        ctx.export("someOutput", aResource.result());
    }
}
```
{{% /choosable %}}
{{< /chooser >}}


## Configuration Reference

The following configuration inputs are supported:
