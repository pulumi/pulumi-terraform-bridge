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

```hcl
resource "outscale_volume" "volume01" {
	subregion_name = "${var.region}a"
	size           = 10
	iops           = 100
	volume_type    = "io1"
}
```

### Creating a snapshot before volume deletion

```hcl
resource "outscale_volume" "volume01" {
    termination_snapshot_name = "deleting_volume_snap"     
    subregion_name = "${var.region}a"
    size           = 40
}
``````

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
