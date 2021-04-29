Preamble

### Nested Schema for `widget`

Optional:

- **group_definition** (Block List, Max: 1) The definition for a Group widget. (see [below for nested schema](#nestedblock--widget--group_definition))

### Nested Schema for `widget.group_definition`

Required:

- **layout_type** (String) The layout type of the group, only 'ordered' for now.
- **widget** (Block List, Min: 1) The list of widgets in this group. (see [below for nested schema](#nestedblock--widget--group_definition--widget))

Optional:

- **title**  (String)

### Nested Schema for `widget.group_definition.widget`

Optional:

- **change_definition** (Block List, Max: 1) The definition for a Change  widget. (see [below for nested schema](#nestedblock--widget--group_definition--widget--change_definition))

### Nested Schema for `widget.group_definition.widget.change_definition`

Optional:

- **custom_link** (Block List) Nested block describing a custom link. Multiple `custom_link` blocks are allowed with the structure below. (see [below for nested schema](#nestedblock--widget--group_definition--widget--change_definition--custom_link))
- **live_span** (String) The timeframe to use when displaying the widget. One of `10m`, `30m`, `1h`, `4h`, `1d`, `2d`, `1w`, `1mo`, `3mo`, `6mo`, `1y`, `alert`.
- **request** (Block List) Nested block describing the request to use when displaying the widget. Multiple request blocks are allowed with the structure below (exactly one of `q`, `apm_query`, `log_query`, `rum_query`, `security_query` or `process_query` is required within the request block). (see [below for nested schema](#nestedblock--widget--group_definition--widget--change_definition--request))
- **time** (Map of String, Deprecated) Nested block describing the timeframe to use when displaying the widget. The structure of this block is described below. **Deprecated.** Define `live_span` directly in the widget definition instead.
- **title** (String) The title of the widget.
- **title_align** (String) The alignment of the widget's title. One of `left`, `center`, or `right`.
- **title_size** (String) The size of the widget's title. Default is 16.


### Nested Schema for `widget.group_definition.widget.change_definition.custom_link`

Required:

- **label** (String) The label for the custom link URL.
- **link** (String) The URL of the custom link.

<a id="nestedblock--widget--group_definition--widget--id--request"></a>


### Nested Schema for `widget.group_definition.widget.change_definition.request`

Optional:

- **aggregator** (String) The aggregator to use for time aggregation. One of `avg`, `min`, `max`, `sum`, `last`.
- **alias** (String) The alias for the column name. Default is the metric name.
