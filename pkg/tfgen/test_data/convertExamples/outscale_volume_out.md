---
layout: "outscale"
page_title: "OUTSCALE: outscale_volume"
sidebar_current: "outscale-volume"
description: |-
  [Manages a volume.]
---

# outscale_volume Resource

Manages a volume.

For more information on this resource, see the [User Guide](https://docs.outscale.com/en/userguide/About-Volumes.html).  
For more information on this resource actions, see the [API documentation](https://docs.outscale.com/api#3ds-outscale-api-volume).

## Example Usage

### Creating an io1 volume

<!--Start PulumiCodeChooser -->
```typescript
import * as pulumi from "@pulumi/pulumi";
import * as outscale from "@pulumi/outscale";

const volume01 = new outscale.index.Outscale_volume("volume01", {
    subregionName: `${_var.region}a`,
    size: 10,
    iops: 100,
    volumeType: "io1",
});
```
```python
import pulumi
import pulumi_outscale as outscale

volume01 = outscale.index.Outscale_volume("volume01",
    subregion_name=f{var.region}a,
    size=10,
    iops=100,
    volume_type=io1)
```
```csharp
using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Outscale = Pulumi.Outscale;

return await Deployment.RunAsync(() => 
{
    var volume01 = new Outscale.Index.Outscale_volume("volume01", new()
    {
        SubregionName = $"{@var.Region}a",
        Size = 10,
        Iops = 100,
        VolumeType = "io1",
    });

});
```
```go
package main

import (
	"fmt"

	"github.com/pulumi/pulumi-outscale/sdk/go/outscale"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := outscale.NewOutscale_volume(ctx, "volume01", &outscale.Outscale_volumeArgs{
			SubregionName: fmt.Sprintf("%va", _var.Region),
			Size:          10,
			Iops:          100,
			VolumeType:    "io1",
		})
		if err != nil {
			return err
		}
		return nil
	})
}
```
```java
package generated_program;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.core.Output;
import com.pulumi.outscale.outscale_volume;
import com.pulumi.outscale.outscale_volumeArgs;
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
        var volume01 = new Outscale_volume("volume01", Outscale_volumeArgs.builder()
            .subregionName(String.format("%sa", var_.region()))
            .size(10)
            .iops(100)
            .volumeType("io1")
            .build());

    }
}
```
```yaml
resources:
  volume01:
    type: outscale:outscale_volume
    properties:
      subregionName: ${var.region}a
      size: 10
      iops: 100
      volumeType: io1
```
<!--End PulumiCodeChooser -->

### Creating a snapshot before volume deletion

<!--Start PulumiCodeChooser -->
```typescript
import * as pulumi from "@pulumi/pulumi";
import * as outscale from "@pulumi/outscale";

const volume01 = new outscale.index.Outscale_volume("volume01", {
    terminationSnapshotName: "deleting_volume_snap",
    subregionName: `${_var.region}a`,
    size: 40,
});
```
```python
import pulumi
import pulumi_outscale as outscale

volume01 = outscale.index.Outscale_volume("volume01",
    termination_snapshot_name=deleting_volume_snap,
    subregion_name=f{var.region}a,
    size=40)
```
```csharp
using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Outscale = Pulumi.Outscale;

return await Deployment.RunAsync(() => 
{
    var volume01 = new Outscale.Index.Outscale_volume("volume01", new()
    {
        TerminationSnapshotName = "deleting_volume_snap",
        SubregionName = $"{@var.Region}a",
        Size = 40,
    });

});
```
```go
package main

import (
	"fmt"

	"github.com/pulumi/pulumi-outscale/sdk/go/outscale"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := outscale.NewOutscale_volume(ctx, "volume01", &outscale.Outscale_volumeArgs{
			TerminationSnapshotName: "deleting_volume_snap",
			SubregionName:           fmt.Sprintf("%va", _var.Region),
			Size:                    40,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
```
```java
package generated_program;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.core.Output;
import com.pulumi.outscale.outscale_volume;
import com.pulumi.outscale.outscale_volumeArgs;
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
        var volume01 = new Outscale_volume("volume01", Outscale_volumeArgs.builder()
            .terminationSnapshotName("deleting_volume_snap")
            .subregionName(String.format("%sa", var_.region()))
            .size(40)
            .build());

    }
}
```
```yaml
resources:
  volume01:
    type: outscale:outscale_volume
    properties:
      terminationSnapshotName: deleting_volume_snap
      subregionName: ${var.region}a
      size: 40
```
<!--End PulumiCodeChooser -->

## Argument Reference

The following arguments are supported:

* `iops` - (Optional) The number of I/O operations per second (IOPS). This parameter must be specified only if you create an `io1` volume. The maximum number of IOPS allowed for `io1` volumes is `13000` with a maximum performance ratio of 300 IOPS per gibibyte.
* `size` - (Optional) The size of the volume, in gibibytes (GiB). The maximum allowed size for a volume is 14901 GiB. This parameter is required if the volume is not created from a snapshot (`snapshot_id` unspecified).
* `snapshot_id` - (Optional) The ID of the snapshot from which you want to create the volume.
* `subregion_name` - (Required) The Subregion in which you want to create the volume.
* `tags` - (Optional) A tag to add to this resource. You can specify this argument several times.
    * `key` - (Required) The key of the tag, with a minimum of 1 character.
    * `value` - (Required) The value of the tag, between 0 and 255 characters.
* `termination_snapshot_name` - (Optional) Whether you want to create a snapshot before the volume deletion.
* `volume_type` - (Optional) The type of volume you want to create (`io1` \| `gp2` \| `standard`). If not specified, a `standard` volume is created.<br />
For more information about volume types, see [About Volumes > Volume Types and IOPS](https://docs.outscale.com/en/userguide/About-Volumes.html#_volume_types_and_iops).

## Attribute Reference

The following attributes are exported:

* `creation_date` - The date and time (UTC) at which the volume was created.
* `iops` - The number of I/O operations per second (IOPS):<br />- For `io1` volumes, the number of provisioned IOPS.<br />- For `gp2` volumes, the baseline performance of the volume.
* `linked_volumes` - Information about your volume attachment.
    * `delete_on_vm_deletion` - If true, the volume is deleted when terminating the VM. If false, the volume is not deleted when terminating the VM.
    * `device_name` - The name of the device.
    * `state` - The state of the attachment of the volume (`attaching` \| `detaching` \| `attached` \| `detached`).
    * `vm_id` - The ID of the VM.
    * `volume_id` - The ID of the volume.
* `size` - The size of the volume, in gibibytes (GiB).
* `snapshot_id` - The snapshot from which the volume was created.
* `state` - The state of the volume (`creating` \| `available` \| `in-use` \| `updating` \| `deleting` \| `error`).
* `subregion_name` - The Subregion in which the volume was created.
* `tags` - One or more tags associated with the volume.
    * `key` - The key of the tag, with a minimum of 1 character.
    * `value` - The value of the tag, between 0 and 255 characters.
* `volume_id` - The ID of the volume.
* `volume_type` - The type of the volume (`standard` \| `gp2` \| `io1`).

## Import

A volume can be imported using its ID. For example:

```console

$ terraform import outscale_volume.ImportedVolume vol-12345678

```
