---
layout: "signalfx"
page_title: "Splunk Observability Cloud: signalfx_log_timeline"
sidebar_current: "docs-signalfx-resource-log-timeline"
description: |-
  Allows Terraform to create and manage log timelines in Splunk Observability Cloud
---

# signalfx_log_timeline

You can add logs data to your Observability Cloud dashboards without turning your logs into metrics first.

A log timeline chart displays timeline visualization in a dashboard and shows you in detail what is happening and why.

## Example

```tf
resource "signalfx_log_timeline" "my_log_timeline" {
  name        = "Sample Log Timeline"
  description = "Lorem ipsum dolor sit amet, laudem tibique iracundia at mea. Nam posse dolores ex, nec cu adhuc putent honestatis"

  program_text = <<-EOF
  logs(filter=field('message') == 'Transaction processed' and field('service.name') == 'paymentservice').publish()
  EOF

  time_range = 900

}
```

## Arguments

The following arguments are supported in the resource block:

* `name` - (Required) Name of the log timeline.
* `program_text` - (Required) Signalflow program text for the log timeline. More info at https://dev.splunk.com/observability/docs/.
* `description` - (Optional) Description of the log timeline.
* `time_range` - (Optional) From when to display data. Splunk Observability Cloud time syntax (e.g. `"-5m"`, `"-1h"`). Conflicts with `start_time` and `end_time`.
* `start_time` - (Optional) Seconds since epoch. Used for visualization. Conflicts with `time_range`.
* `end_time` - (Optional) Seconds since epoch. Used for visualization. Conflicts with `time_range`.
* `default_connection` - (Optional) The connection that the log timeline uses to fetch data. This could be Splunk Enterprise, Splunk Enterprise Cloud or Observability Cloud.

## Attributes

In a addition to all arguments above, the following attributes are exported:

* `id` - The ID of the log timeline.
* `url` - The URL of the log timeline.